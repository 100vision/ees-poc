//go:build windows

// Command agent runs the EES Windows Service.
// It listens on \\.\pipe\ees, verifies executables against the whitelist,
// and elevates approved programs via CreateProcessAsUser.
//
// Usage:
//
//	ees-agent.exe install     Install the Windows service
//	ees-agent.exe uninstall   Uninstall the Windows service
//	ees-agent.exe debug       Run as console app (for testing)
//
// Build (cross-compile from WSL2):
//
//	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/ees-agent.exe .
package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc"
)

const serviceName = "EESAgent"

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install":
			if err := installService(serviceName, "Enterprise Elevation Service Agent"); err != nil {
				fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ EES Agent service installed")
			return

		case "uninstall":
			if err := uninstallService(serviceName); err != nil {
				fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ EES Agent service removed")
			return

		case "debug":
			// Run as console app for debugging
			runAgent(nil)
			return
		}
	}

	// Run as a Windows service
	if err := svc.Run(serviceName, &agentService{}); err != nil {
		fmt.Fprintf(os.Stderr, "Service run failed: %v\n", err)
		os.Exit(1)
	}
}
