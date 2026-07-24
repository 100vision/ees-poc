//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"ees-demo/common/log"
)

// Elevation constants.
const (
	tokenLinkedTokenClass    = 19
	createUnicodeEnvironment = 0x00000400
	createNewConsole         = 0x00000010
)

// ElevationEngine handles the Windows elevation chain:
//
//	WTSGetActiveConsoleSessionId
//	→ WTSQueryUserToken
//	→ DuplicateTokenEx
//	→ Branch on user type:
//	   • Administrator → GetLinkedToken (UAC elevation)
//	   • Standard user → OpenProcessToken(SYSTEM) + SetSessionId
//	→ CreateEnvironmentBlock
//	→ CreateProcessAsUser
//
// The engine is designed to run from a SYSTEM context (Windows Service) and
// launches a process on the active user's desktop with elevated privileges:
// administrators get their linked UAC token, standard users get a SYSTEM token
// with the active session ID set on it.
type ElevationEngine struct {
	logger *log.Logger
}

// NewElevationEngine creates an elevation engine with the given logger.
func NewElevationEngine(logger *log.Logger) *ElevationEngine {
	return &ElevationEngine{logger: logger}
}

// Launch elevates the specified program to run on the active user's desktop.
// It returns the process exit code on success, or an error describing the failure.
func (e *ElevationEngine) Launch(path string) (uint32, error) {
	e.logger.Info("Elevation start: %s", path)

	// Step 1: Get the active console session ID
	sessionID, err := e.getActiveConsoleSessionId()
	if err != nil {
		return 0, fmt.Errorf("step 1 (session ID): %w", err)
	}
	e.logger.Info("  Session ID: %d", sessionID)

	// Step 2: Query user token for this session (requires SYSTEM)
	token, err := e.wtsQueryUserToken(sessionID)
	if err != nil {
		return 0, fmt.Errorf("step 2 (user token): %w", err)
	}
	e.logger.Info("  User token obtained")

	// Step 3: Convert impersonation token to primary token
	primaryToken, err := e.duplicateTokenAsPrimary(token)
	token.Close()
	if err != nil {
		return 0, fmt.Errorf("step 3 (duplicate token): %w", err)
	}
	e.logger.Info("  Primary token obtained")

	// Step 4: Determine user type and branch elevation strategy
	//   - Administrator → GetLinkedToken → CreateProcessAsUser (admin privileges)
	//   - Standard user → SYSTEM token → SetSessionID → CreateProcessAsUser (SYSTEM privileges)
	isAdmin, err := e.isAdminToken(primaryToken)
	if err != nil {
		e.logger.Warn("  Admin check failed (assuming admin): %v", err)
		isAdmin = true
	}

	var execToken windows.Token // token used to launch the process
	var envToken windows.Token  // token used to create the env block

	if isAdmin {
		// --- Administrator user path ---
		e.logger.Info("  User type: Administrator")
		linked, err := e.getLinkedToken(primaryToken)
		if err != nil {
			e.logger.Info("  Linked token unavailable (UAC disabled?), using primary token")
			execToken = primaryToken
		} else {
			e.logger.Info("  Linked (elevated) token obtained")
			primaryToken.Close()
			execToken = linked
		}
		envToken = execToken
	} else {
		// --- Standard user path ---
		e.logger.Info("  User type: Standard user — using SYSTEM token for elevation")
		sysToken, err := e.getSystemTokenForSession(sessionID)
		if err != nil {
			primaryToken.Close()
			return 0, fmt.Errorf("step 4 (SYSTEM token): %w", err)
		}
		envToken = primaryToken // user env block preserves user profile vars
		execToken = sysToken
	}

	// Step 5: Create environment block for the target user
	env, err := e.createEnvironmentBlock(envToken)
	if err != nil {
		execToken.Close()
		if !isAdmin {
			primaryToken.Close()
		}
		return 0, fmt.Errorf("step 5 (env block): %w", err)
	}
	if !isAdmin {
		primaryToken.Close() // env block done, user token no longer needed
	}

	// Step 6: Launch the process on the user's desktop
	e.logger.Info("  Launching: %s", path)
	exitCode, err := e.createProcessAsUser(execToken, path, env)
	e.destroyEnvironmentBlock(env)
	execToken.Close()

	if err != nil {
		return 0, fmt.Errorf("step 6 (launch): %w", err)
	}

	e.logger.Info("Elevation complete (exit code: %d)%s", exitCode, exitCodeHint(exitCode))
	return exitCode, nil
}

// getActiveConsoleSessionId returns the session ID of the currently active
// console session (the user at the physical console).
func (e *ElevationEngine) getActiveConsoleSessionId() (uint32, error) {
	sessionID := windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return 0, fmt.Errorf("no active console session")
	}
	return sessionID, nil
}

// wtsQueryUserToken retrieves the primary access token of the logged-on user
// in the specified session. Must be called from a SYSTEM context.
func (e *ElevationEngine) wtsQueryUserToken(sessionID uint32) (windows.Token, error) {
	var token windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &token); err != nil {
		return 0, fmt.Errorf("WTSQueryUserToken(session=%d): %w", sessionID, err)
	}
	return token, nil
}

// duplicateTokenAsPrimary converts an impersonation token to a primary token.
func (e *ElevationEngine) duplicateTokenAsPrimary(token windows.Token) (windows.Token, error) {
	const (
		tokenPrimary               = 1
		securityImpersonationLevel = 2
	)

	var primaryToken windows.Token
	err := windows.DuplicateTokenEx(
		token,
		windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_ALL_ACCESS,
		nil,
		securityImpersonationLevel,
		tokenPrimary,
		&primaryToken,
	)
	if err != nil {
		return 0, fmt.Errorf("DuplicateTokenEx: %w", err)
	}
	return primaryToken, nil
}

// getLinkedToken gets the linked (elevated) UAC token. Only works when the
// calling process has SE_TCB_NAME privilege and the target user is an
// administrator with UAC enabled.
func (e *ElevationEngine) getLinkedToken(token windows.Token) (windows.Token, error) {
	var linkedToken windows.Token
	var returnLen uint32

	err := windows.GetTokenInformation(
		token,
		uint32(tokenLinkedTokenClass),
		(*byte)(unsafe.Pointer(&linkedToken)),
		uint32(unsafe.Sizeof(linkedToken)),
		&returnLen,
	)
	if err != nil {
		return 0, fmt.Errorf("GetTokenInformation(LinkedToken): %w", err)
	}
	return linkedToken, nil
}

// isAdminToken checks whether the given token belongs to a member of the
// Administrators group (BUILTIN\Administrators, S-1-5-32-544).
func (e *ElevationEngine) isAdminToken(token windows.Token) (bool, error) {
	var adminSID *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&adminSID,
	)
	if err != nil {
		return false, fmt.Errorf("AllocateAndInitializeSid: %w", err)
	}
	defer windows.FreeSid(adminSID)

	var isMember bool
	err = windows.CheckTokenMembership(token, adminSID, &isMember)
	if err != nil {
		return false, fmt.Errorf("CheckTokenMembership: %w", err)
	}
	return isMember, nil
}

// getSystemTokenForSession duplicates the current process (SYSTEM) token,
// converts it to a primary token, and sets the session ID so the process
// appears on the correct user desktop when launched with CreateProcessAsUser.
func (e *ElevationEngine) getSystemTokenForSession(sessionID uint32) (windows.Token, error) {
	var procToken windows.Token
	err := windows.OpenProcessToken(
		windows.GetCurrentProcess(),
		windows.TOKEN_ALL_ACCESS,
		&procToken,
	)
	if err != nil {
		return 0, fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer procToken.Close()

	var primaryToken windows.Token
	err = windows.DuplicateTokenEx(
		procToken,
		windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_ALL_ACCESS,
		nil,
		2, // SecurityImpersonation
		1, // TokenPrimary
		&primaryToken,
	)
	if err != nil {
		return 0, fmt.Errorf("DuplicateTokenEx: %w", err)
	}

	// Set session ID so the process launches on the user's desktop, not Session 0.
	const tokenSessionID = 32
	err = windows.SetTokenInformation(
		primaryToken,
		tokenSessionID,
		(*byte)(unsafe.Pointer(&sessionID)),
		uint32(unsafe.Sizeof(sessionID)),
	)
	if err != nil {
		primaryToken.Close()
		return 0, fmt.Errorf("SetTokenInformation(SessionId=%d): %w", sessionID, err)
	}

	return primaryToken, nil
}

// createEnvironmentBlock creates an environment block for the specified token.
func (e *ElevationEngine) createEnvironmentBlock(token windows.Token) (*uint16, error) {
	var env *uint16
	err := windows.CreateEnvironmentBlock(&env, token, false)
	if err != nil {
		return nil, fmt.Errorf("CreateEnvironmentBlock: %w", err)
	}
	return env, nil
}

// destroyEnvironmentBlock frees an environment block created by createEnvironmentBlock.
func (e *ElevationEngine) destroyEnvironmentBlock(env *uint16) {
	_ = windows.DestroyEnvironmentBlock(env)
}

// exitCodeHint returns a human-readable description for common installer exit codes.
func exitCodeHint(code uint32) string {
	switch code {
	case 0:
		return " (installer completed successfully)"
	case 1:
		return " (installer: user cancelled or installation failed)"
	case 2:
		return " (installer: invalid parameter or file not found)"
	case 3:
		return " (installer: access denied or insufficient privileges)"
	case 1602:
		return " (Windows Installer: user cancelled installation)"
	case 1603:
		return " (Windows Installer: fatal error during installation)"
	case 1641:
		return " (Windows Installer: restart required)"
	default:
		return ""
	}
}

// createProcessAsUser launches the specified program under the given token
// on the user's desktop (winsta0\default) and waits for it to exit.
func (e *ElevationEngine) createProcessAsUser(token windows.Token, appPath string, env *uint16) (uint32, error) {
	appPathPtr, err := windows.UTF16PtrFromString(appPath)
	if err != nil {
		return 0, fmt.Errorf("UTF16: %w", err)
	}

	si := &windows.StartupInfo{
		Cb:         uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop:    windows.StringToUTF16Ptr("winsta0\\default"),
		Flags:      windows.STARTF_USESHOWWINDOW,
		ShowWindow: windows.SW_SHOWNORMAL,
	}
	pi := &windows.ProcessInformation{}

	cmdLine, _ := windows.UTF16PtrFromString(appPath)

	err = windows.CreateProcessAsUser(
		token,
		appPathPtr,
		cmdLine,
		nil, // process attributes
		nil, // thread attributes
		false,
		createUnicodeEnvironment|createNewConsole,
		env,
		nil, // current directory
		si,
		pi,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateProcessAsUser: %w", err)
	}

	windows.WaitForSingleObject(pi.Process, windows.INFINITE)

	var exitCode uint32
	windows.GetExitCodeProcess(pi.Process, &exitCode)

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return exitCode, nil
}
