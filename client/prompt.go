//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"ees-demo/common/constants"
	"ees-demo/common/types"
)

// Windows MessageBox constants.
const (
	mbOK        = 0x00000000
	mbIconInfo  = 0x00000040
	mbIconError = 0x00000010
	mbTopMost   = 0x00040000
	mbSetForeground = 0x00010000
)

var (
	moduser32      = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW = moduser32.NewProc("MessageBoxW")
)

// showResult displays the Elevation result in a Windows MessageBox.
func showResult(resp *types.Response) {
	title := "Enterprise Elevation Service"
	var message string
	flags := uintptr(mbOK | mbTopMost | mbSetForeground)

	switch resp.Result {
	case constants.ResultAllow:
		message = fmt.Sprintf("✅ %s\n\nThe application has been elevated successfully.", resp.Message)
		flags |= mbIconInfo
	case constants.ResultDeny:
		message = fmt.Sprintf("🚫 %s\n\nContact your IT administrator if you need this application.", resp.Message)
		flags |= mbIconError
	case constants.ResultError:
		message = fmt.Sprintf("❌ %s\n\nPlease ensure the EES Agent service is running.", resp.Message)
		flags |= mbIconError
	default:
		message = fmt.Sprintf("Unknown response: %s - %s", resp.Result, resp.Message)
		flags |= mbIconError
	}

	titlePtr, _ := windows.UTF16PtrFromString(title)
	messagePtr, _ := windows.UTF16PtrFromString(message)

	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags,
	)
}

// showError displays an error message in a MessageBox.
func showError(title, message string) {
	showMsgBox(title, message, mbIconError)
}

// showInfo displays an informational message in a MessageBox.
func showInfo(title, message string) {
	showMsgBox(title, message, mbIconInfo)
}

// showMsgBox shows a Windows MessageBox with the given title, message, and icon.
func showMsgBox(title, message string, icon uintptr) {
	flags := uintptr(mbOK | mbTopMost | mbSetForeground) | icon
	titlePtr, _ := windows.UTF16PtrFromString(title)
	messagePtr, _ := windows.UTF16PtrFromString(message)
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags,
	)
}
