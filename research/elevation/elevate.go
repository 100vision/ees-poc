//go:build windows

package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows API constants.
const (
	tokenLinkedTokenClass = 19 // TokenLinkedToken information class for GetTokenInformation

	createUnicodeEnvironment = 0x00000400
	createNewConsole         = 0x00000010
)

// hasPathSeparator checks whether path contains a directory separator.
func hasPathSeparator(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' || path[i] == '/' {
			return true
		}
	}
	return false
}

// getActiveConsoleSessionId returns the session ID of the currently active
// console session (the user at the physical console).
//
// Uses windows.WTSGetActiveConsoleSessionId which loads from kernel32.dll
// (not wtsapi32.dll — the latter broke on Win11 24H2+).
func getActiveConsoleSessionId() (uint32, error) {
	sessionID := windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return 0, fmt.Errorf("WTSGetActiveConsoleSessionId: no active console session")
	}
	return sessionID, nil
}

// wtsQueryUserToken retrieves the primary access token of the logged-on user
// in the specified session. Must be called from a SYSTEM context (e.g. a service).
func wtsQueryUserToken(sessionID uint32) (windows.Token, error) {
	var token windows.Token
	err := windows.WTSQueryUserToken(sessionID, &token)
	if err != nil {
		return 0, fmt.Errorf("WTSQueryUserToken(session=%d): %w", sessionID, err)
	}
	return token, nil
}

// wtsSessionInfo is an alias for the Go syscall-provided type.
type wtsSessionInfo = windows.WTS_SESSION_INFO

// wtsEnumerateSessions lists all terminal server sessions on the local machine.
func wtsEnumerateSessions() ([]wtsSessionInfo, error) {
	var pInfo *windows.WTS_SESSION_INFO
	var count uint32

	err := windows.WTSEnumerateSessions(
		0,       // WTS_CURRENT_SERVER_HANDLE
		0,       // Reserved
		1,       // Version (must be 1)
		&pInfo,
		&count,
	)
	if err != nil {
		return nil, fmt.Errorf("WTSEnumerateSessions: %w", err)
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(pInfo)))

	sessions := unsafe.Slice(pInfo, count)
	result := make([]wtsSessionInfo, len(sessions))
	for i, s := range sessions {
		result[i] = s
	}
	return result, nil
}

// createEnvironmentBlock creates an environment block for the specified token.
func createEnvironmentBlock(token windows.Token) (*uint16, error) {
	var env *uint16
	err := windows.CreateEnvironmentBlock(&env, token, false)
	if err != nil {
		return nil, fmt.Errorf("CreateEnvironmentBlock: %w", err)
	}
	return env, nil
}

// destroyEnvironmentBlock frees an environment block.
func destroyEnvironmentBlock(env *uint16) {
	_ = windows.DestroyEnvironmentBlock(env)
}

// duplicateTokenAsPrimary converts an impersonation token to a primary token
// that can be used with CreateProcessAsUser.
func duplicateTokenAsPrimary(token windows.Token) (windows.Token, error) {
	const (
		tokenPrimary            = 1
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

// getLinkedToken gets the linked (elevated) token from UAC. Only works when
// the calling process has SE_TCB_NAME privilege and the target user is an
// administrator with UAC enabled.
func getLinkedToken(token windows.Token) (windows.Token, error) {
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
		return 0, fmt.Errorf("GetTokenInformation(TokenLinkedToken=19): %w", err)
	}
	return linkedToken, nil
}

// launchProcessAsUser creates a new process under the specified user token
// and waits for it to exit. The process runs on winsta0\default (user desktop).
func launchProcessAsUser(token windows.Token, appPath string, env *uint16) (uint32, error) {
	appPathPtr, err := windows.UTF16PtrFromString(appPath)
	if err != nil {
		return 0, fmt.Errorf("UTF16PtrFromString: %w", err)
	}

	si := &windows.StartupInfo{
		Cb:        uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop:   windows.StringToUTF16Ptr("winsta0\\default"),
		Flags:     windows.STARTF_USESHOWWINDOW,
		ShowWindow: windows.SW_SHOWNORMAL,
	}
	pi := &windows.ProcessInformation{}

	cmdLine, _ := windows.UTF16PtrFromString(appPath)

	err = windows.CreateProcessAsUser(
		token,
		appPathPtr,
		cmdLine,
		nil,                                             // process attributes
		nil,                                             // thread attributes
		false,                                           // inherit handles
		createUnicodeEnvironment|createNewConsole,
		env,                                             // environment block
		nil,                                             // current directory
		si,
		pi,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateProcessAsUser(%s): %w", appPath, err)
	}

	windows.WaitForSingleObject(pi.Process, windows.INFINITE)

	var exitCode uint32
	windows.GetExitCodeProcess(pi.Process, &exitCode)

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return exitCode, nil
}

// launchForActiveSession is the main entry point: it gets the active session's
// token, optionally elevates it, and launches the specified program.
func launchForActiveSession(appPath string) {
	if appPath == "" {
		appPath = "C:\\Windows\\System32\\notepad.exe"
	}

	// If the path has no directory separator, resolve it from System32.
	// (CreateProcessAsUser with environment block doesn't inherit the
	// SYSTEM account's PATH, so bare executable names won't be found.)
	if !hasPathSeparator(appPath) {
		resolved := "C:\\Windows\\System32\\" + appPath
		fmt.Printf("(Resolved bare name %q → %q)\n", appPath, resolved)
		appPath = resolved
	}

	fmt.Println("=== EES Elevation Pre-Research ===")
	fmt.Printf("Target: %s\n\n", appPath)

	// Step 1: Get active console session ID
	sessionID, err := getActiveConsoleSessionId()
	if err != nil {
		fmt.Printf("✗ Step 1 — WTSGetActiveConsoleSessionId: FAILED\n  %v\n", err)
		fmt.Println("\nTIP: Make sure a user is logged in at the console.")
		os.Exit(1)
	}
	fmt.Printf("✓ Step 1 — Active Console Session ID: %d\n", sessionID)

	// Step 2: Query user token for this session
	userToken, err := wtsQueryUserToken(sessionID)
	if err != nil {
		fmt.Printf("✗ Step 2 — WTSQueryUserToken: FAILED\n  %v\n", err)
		fmt.Println("\nTIP: This step requires SE_TCB_NAME privilege (SYSTEM account).")
		fmt.Println("Run from NT AUTHORITY\\SYSTEM (e.g., psexec -s -i cmd.exe).")
		os.Exit(1)
	}
	fmt.Printf("✓ Step 2 — WTSQueryUserToken: Token obtained\n")

	// Step 3: Duplicate to primary token
	primaryToken, err := duplicateTokenAsPrimary(userToken)
	userToken.Close()
	if err != nil {
		fmt.Printf("✗ Step 3 — DuplicateTokenEx: FAILED\n  %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Step 3 — DuplicateTokenEx: Primary token obtained\n")

	// Step 4 (optional): Try to get elevated (linked) token
	elevatedToken, err := getLinkedToken(primaryToken)
	if err != nil {
		fmt.Printf("— Step 4 — GetLinkedToken: SKIPPED (%v)\n", err)
		fmt.Println("  (Using primary token — process may launch with filtered privileges)")
		elevatedToken = primaryToken
	} else {
		fmt.Printf("✓ Step 4 — GetLinkedToken: Elevated token obtained\n")
		primaryToken.Close()
	}

	// Step 5: Create environment block
	env, err := createEnvironmentBlock(elevatedToken)
	if err != nil {
		fmt.Printf("✗ Step 5 — CreateEnvironmentBlock: FAILED\n  %v\n", err)
		elevatedToken.Close()
		os.Exit(1)
	}
	fmt.Printf("✓ Step 5 — CreateEnvironmentBlock: Environment created\n")

	// Step 6: Launch process
	fmt.Printf("\n→ Step 6 — Launching: %s\n", appPath)
	exitCode, err := launchProcessAsUser(elevatedToken, appPath, env)
	destroyEnvironmentBlock(env)
	elevatedToken.Close()

	if err != nil {
		fmt.Printf("✗ Step 6 — CreateProcessAsUser: FAILED\n  %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Step 6 — Process exited with code: %d\n", exitCode)

	fmt.Println("\n=== Elevation chain: VERIFIED ===")
	fmt.Println("The program should have appeared on your desktop (not Session 0).")
	fmt.Println("If it did, the EES elevation approach is technically feasible.")
}

// listSessions enumerates all terminal sessions and prints them.
func listSessions() {
	fmt.Println("Terminal Sessions:")
	fmt.Println("Session ID | State    | Station Name")
	fmt.Println("-----------+----------+--------------")

	sessions, err := wtsEnumerateSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enumerating sessions: %v\n", err)
		os.Exit(1)
	}

	for _, s := range sessions {
		state := "unknown"
		switch s.State {
		case 0:
			state = "Active"
		case 1:
			state = "Connected"
		case 2:
			state = "ConnectQuery"
		case 3:
			state = "Shadow"
		case 4:
			state = "Disconnected"
		case 5:
			state = "Idle"
		case 6:
			state = "Listen"
		case 7:
			state = "Reset"
		case 8:
			state = "Down"
		case 9:
			state = "Init"
		}

		stationName := windows.UTF16PtrToString(s.WindowStationName)
		fmt.Printf("  %-9d | %-8s | %s\n", s.SessionID, state, stationName)
	}
}
