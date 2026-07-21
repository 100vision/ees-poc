//go:build windows

// Command client registers the "Run with Enterprise Admin" Explorer context menu
// and communicates with the EES Agent via Named Pipe.
//
// Usage:
//
//	client.exe install              Register context menu (admin)
//	client.exe uninstall            Unregister context menu (admin)
//	client.exe "C:\path\to\file"    Send file path to Agent for elevation
//
// Build (cross-compile from WSL2):
//
//	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/ees-client.exe .
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "install":
		if err := installMenu(); err != nil {
			showError("Install Failed", err.Error())
			os.Exit(1)
		}
		showInfo("Context Menu Installed",
			"'Run with Enterprise Admin' is now available for .exe files in Explorer.")

	case "uninstall":
		if err := uninstallMenu(); err != nil {
			showError("Uninstall Failed", err.Error())
			os.Exit(1)
		}
		showInfo("Context Menu Removed", "")

	default:
		// os.Args[1] is the file path passed by Explorer context menu
		path := os.Args[1]
		if err := verifyAndElevate(path); err != nil {
			showError("Elevation Error", err.Error())
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("EES Client — Enterprise Elevation Service")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  client.exe install                  Register Explorer context menu")
	fmt.Println("  client.exe uninstall                Remove Explorer context menu")
	fmt.Println(`  client.exe "C:\path\to\setup.exe"    Send file to Agent for elevation`)
}
