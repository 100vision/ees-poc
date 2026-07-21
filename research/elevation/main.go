//go:build windows

// Command elevation tests the Windows elevation chain:
//
//   WTSGetActiveConsoleSessionId
//   → WTSQueryUserToken
//   → DuplicateTokenEx
//   → CreateEnvironmentBlock
//   → CreateProcessAsUser
//
// Build (cross-compile from WSL2):
//
//	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ../../build/ees-elevation.exe .
//
// Run on Windows test machine:
//
//	# As administrator (to test from SYSTEM/service context):
//	elevation.exe -session
//
//	# Launch a specific program:
//	elevation.exe -path C:\Windows\notepad.exe
//
//	# List active sessions on the machine:
//	elevation.exe -list
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	session := flag.Bool("session", false, "Run elevation test using active session (simulates Service → User flow)")
	path := flag.String("path", "", "Path to executable to launch (default: notepad.exe)")
	list := flag.Bool("list", false, "List active terminal sessions")
	flag.Parse()

	switch {
	case *list:
		listSessions()
	case *session:
		launchForActiveSession(*path)
	default:
		fmt.Println("Usage: elevation.exe -session | -path <exe> | -list")
		fmt.Println("  -session          Test elevation from SYSTEM/service to active user session")
		fmt.Println("  -path   <exe>     Launch a specific executable under the active user (default: notepad.exe)")
		fmt.Println("  -list             Show active terminal sessions")
		os.Exit(1)
	}
}
