//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/sys/windows"

	"ees-demo/common/types"
)

// Named Pipe client connection timeout.
const pipeTimeout = 5 * time.Second

// sendRequest connects to the Agent's Named Pipe, sends a verification request
// for the given file path, and returns the Agent's response.
func sendRequest(path string) (*types.Response, error) {
	pipeName := `\\.\pipe\ees`

	// Open the named pipe (CreateFile)
	pipeHandle, err := openPipe(pipeName)
	if err != nil {
		return nil, fmt.Errorf("connect to Agent: %w", err)
	}
	defer windows.CloseHandle(pipeHandle)

	// Build and send the request
	req := types.Request{Path: path}
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Append newline as message delimiter
	reqData = append(reqData, '\n')

	var bytesWritten uint32
	err = windows.WriteFile(pipeHandle, reqData, &bytesWritten, nil)
	if err != nil {
		return nil, fmt.Errorf("write to pipe: %w", err)
	}

	if int(bytesWritten) != len(reqData) {
		return nil, fmt.Errorf("pipe write incomplete: %d/%d bytes", bytesWritten, len(reqData))
	}

	// Read response (up to 4096 bytes)
	buf := make([]byte, 4096)
	var bytesRead uint32
	err = windows.ReadFile(pipeHandle, buf, &bytesRead, nil)
	if err != nil {
		return nil, fmt.Errorf("read from pipe: %w", err)
	}

	// Parse response JSON
	var resp types.Response
	if err := json.Unmarshal(buf[:bytesRead], &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w (raw: %q)", err, string(buf[:bytesRead]))
	}

	return &resp, nil
}

// openPipe opens a connection to the specified named pipe.
// It sets up the SECURITY_ATTRIBUTES and timeout for the connection.
func openPipe(name string) (windows.Handle, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, fmt.Errorf("UTF16 encode pipe name: %w", err)
	}

	// Try to open the pipe with a timeout loop
	deadline := time.Now().Add(pipeTimeout)
	for time.Now().Before(deadline) {
		handle, err := windows.CreateFile(
			namePtr,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,                         // exclusive access
			nil,                       // default security
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			0,                         // no template
		)
		if err == nil {
			return handle, nil
		}

		// If pipe is busy, wait and retry
		if err == windows.ERROR_PIPE_BUSY {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		return 0, fmt.Errorf("open pipe: %w", err)
	}

	return 0, fmt.Errorf("timeout after %v waiting for pipe %s", pipeTimeout, name)
}

// verifyAndElevate sends the file path to the Agent and shows the result.
func verifyAndElevate(path string) error {
	// Check if file exists
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	_ = attrs // not needed further

	resp, err := sendRequest(path)
	if err != nil {
		return fmt.Errorf("Agent communication failed: %w", err)
	}

	showResult(resp)
	return nil
}
