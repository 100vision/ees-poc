package main

import (
	"encoding/json"
	"fmt"
	"io"
	"unsafe"

	"golang.org/x/sys/windows"

	"ees-demo/common/config"
	"ees-demo/common/constants"
	"ees-demo/common/log"
	"ees-demo/common/types"
)

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")

	procInitializeSecurityDescriptor = modadvapi32.NewProc("InitializeSecurityDescriptor")
	procSetSecurityDescriptorDacl    = modadvapi32.NewProc("SetSecurityDescriptorDacl")
)

func makePipeSecurity() (*windows.SecurityAttributes, error) {
	const secDescRevision = 1

	sd := &windows.SECURITY_DESCRIPTOR{}
	ret, _, _ := procInitializeSecurityDescriptor.Call(
		uintptr(unsafe.Pointer(sd)), secDescRevision,
	)
	if ret == 0 {
		return nil, fmt.Errorf("InitializeSecurityDescriptor failed")
	}

	ret, _, _ = procSetSecurityDescriptorDacl.Call(
		uintptr(unsafe.Pointer(sd)), 1, 0, 0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("SetSecurityDescriptorDacl failed")
	}

	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      1,
	}, nil
}

func runPipeServer(cfg *config.Config, wlPath string, logger *log.Logger, stopChan <-chan struct{}) {
	logger.Info("Pipe server starting on %s", cfg.PipeName)
	logger.Info("Whitelist (hot-reload): %s", wlPath)

	sa, err := makePipeSecurity()
	if err != nil {
		logger.Error("Failed to create pipe security: %v", err)
		return
	}

	for {
		select {
		case <-stopChan:
			logger.Info("Pipe server stopping")
			return
		default:
		}
		if err := handleConnection(cfg, wlPath, sa, logger, stopChan); err != nil {
			logger.Warn("Pipe connection error: %v", err)
		}
	}
}

func handleConnection(cfg *config.Config, wlPath string, sa *windows.SecurityAttributes, logger *log.Logger, stopChan <-chan struct{}) error {
	pipeNamePtr, err := windows.UTF16PtrFromString(cfg.PipeName)
	if err != nil {
		return fmt.Errorf("pipe name UTF16: %w", err)
	}

	pipeHandle, err := windows.CreateNamedPipe(
		pipeNamePtr,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		constants.PipeBufferSize,
		constants.PipeBufferSize,
		0, sa,
	)
	if err != nil {
		return fmt.Errorf("CreateNamedPipe: %w", err)
	}
	defer windows.CloseHandle(pipeHandle)

	logger.Info("Waiting for client connection on %s...", cfg.PipeName)

	type connectResult struct{ err error }
	connectDone := make(chan connectResult, 1)
	go func() {
		err := windows.ConnectNamedPipe(pipeHandle, nil)
		connectDone <- connectResult{err: err}
	}()

	select {
	case <-stopChan:
		windows.CancelIoEx(pipeHandle, nil)
		return fmt.Errorf("pipe server stopping")
	case result := <-connectDone:
		if result.err != nil && result.err != windows.ERROR_PIPE_CONNECTED {
			return fmt.Errorf("ConnectNamedPipe: %w", result.err)
		}
	}

	logger.Info("Client connected")

	req, err := readRequest(pipeHandle)
	if err != nil {
		logger.Error("Read request failed: %v", err)
		sendError(pipeHandle, "Failed to read request")
		return err
	}
	logger.Info("Request: Path=%s", req.Path)

	resp := processRequest(req, wlPath, logger)

	if err := sendResponse(pipeHandle, resp); err != nil {
		logger.Error("Send response failed: %v", err)
		return err
	}

	windows.FlushFileBuffers(pipeHandle)
	windows.DisconnectNamedPipe(pipeHandle)
	logger.Info("Response sent: %s - %s", resp.Result, resp.Message)
	return nil
}

func readRequest(pipeHandle windows.Handle) (*types.Request, error) {
	buf := make([]byte, constants.PipeBufferSize)
	var bytesRead uint32
	err := windows.ReadFile(pipeHandle, buf, &bytesRead, nil)
	if err != nil {
		return nil, fmt.Errorf("ReadFile: %w", err)
	}
	if bytesRead == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	var req types.Request
	if err := json.Unmarshal(buf[:bytesRead], &req); err != nil {
		return nil, fmt.Errorf("JSON parse: %w (raw: %q)", err, string(buf[:bytesRead]))
	}
	if req.Path == "" {
		return nil, fmt.Errorf("empty path in request")
	}
	return &req, nil
}

func sendResponse(pipeHandle windows.Handle, resp *types.Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("JSON marshal: %w", err)
	}
	data = append(data, '\n')
	var bytesWritten uint32
	err = windows.WriteFile(pipeHandle, data, &bytesWritten, nil)
	if err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	return nil
}

func sendError(pipeHandle windows.Handle, message string) {
	_ = sendResponse(pipeHandle, &types.Response{
		Result: constants.ResultError, Message: message,
	})
}

// processRequest handles verification + elevation:
//  1. Verify file exists
//  2. SHA256 + Publisher verification
//  3. Whitelist match (hot-reloaded each request)
//  4. Allow → respond immediately, elevate in background
//  5. Deny / Error → respond with result
func processRequest(req *types.Request, wlPath string, logger *log.Logger) *types.Response {
	// 1. Basic path check
	pathPtr, err := windows.UTF16PtrFromString(req.Path)
	if err != nil {
		return errorResp("Invalid file path")
	}
	_, err = windows.GetFileAttributes(pathPtr)
	if err != nil {
		logger.Warn("File not found: %s", req.Path)
		return errorResp("File not found")
	}

	// 2. Verify file (SHA256 + Publisher)
	v, err := verifyFile(req.Path, logger)
	if err != nil {
		logger.Error("Verification failed: %v", err)
		return errorResp(fmt.Sprintf("Verification failed: %v", err))
	}

	// 3. Load whitelist and match
	wl, err := loadWhitelist(wlPath, nil)
	if err != nil {
		logger.Error("Failed to load whitelist: %v", err)
		return errorResp("Whitelist configuration error")
	}
	resp := wl.decide(v)

	// 4. Allow → respond immediately, launch elevation in background
	if resp.Result == constants.ResultAllow {
		logger.Info("Allow: %s (SHA256=%s, Publisher=%s)", req.Path, v.SHA256, v.Publisher)
		logger.Info("Responding Allow — elevation continuing in background")

		engine := NewElevationEngine(logger)
		go engine.Launch(req.Path)

		return &types.Response{
			Result:  constants.ResultAllow,
			Message: "Elevation Successful",
		}
	}

	// 5. Deny
	logger.Info("Deny: %s (SHA256=%s, Publisher=%s)", req.Path, v.SHA256, v.Publisher)
	return resp
}

func errorResp(message string) *types.Response {
	return &types.Response{
		Result: constants.ResultError, Message: message,
	}
}
