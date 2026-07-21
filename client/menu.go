//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// Registry path for .exe file context menu.
// HKEY_CLASSES_ROOT\exefile\shell\<MenuName>\command
const menuRegistryKey = "exefile\\shell\\Run with Enterprise Admin"

// installMenu registers the "Run with Enterprise Admin" context menu entry
// for .exe files. Requires administrator privileges.
func installMenu() error {
	// Get path to our own executable for the command value
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	commandValue := fmt.Sprintf(`"%s" "%%1"`, exePath)

	// Create the menu key under HKEY_CLASSES_ROOT\exefile\shell
	key, _, err := registry.CreateKey(
		registry.CLASSES_ROOT,
		menuRegistryKey,
		registry.WRITE,
	)
	if err != nil {
		return fmt.Errorf("create registry key: %w", err)
	}
	defer key.Close()

	// Default value = menu display text
	if err := key.SetStringValue("", "Run with Enterprise Admin"); err != nil {
		return fmt.Errorf("set menu title: %w", err)
	}

	// Optional: set icon to our executable
	if err := key.SetStringValue("Icon", exePath+",0"); err != nil {
		return fmt.Errorf("set icon: %w", err)
	}

	// Create the "command" subkey
	cmdKey, _, err := registry.CreateKey(key, "command", registry.WRITE)
	if err != nil {
		return fmt.Errorf("create command key: %w", err)
	}
	defer cmdKey.Close()

	if err := cmdKey.SetStringValue("", commandValue); err != nil {
		return fmt.Errorf("set command: %w", err)
	}

	return nil
}

// uninstallMenu removes the "Run with Enterprise Admin" context menu entry.
func uninstallMenu() error {
	// Delete command subkey first, then parent
	if err := registry.DeleteKey(registry.CLASSES_ROOT, menuRegistryKey+"\\command"); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("delete command key: %w", err)
		}
	}
	if err := registry.DeleteKey(registry.CLASSES_ROOT, menuRegistryKey); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("delete menu key: %w", err)
		}
	}
	return nil
}
