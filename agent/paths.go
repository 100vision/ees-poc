package main

import (
	"os"
	"path/filepath"
)

// agentDir returns the directory containing the agent executable.
// Used to resolve config/log paths relative to the exe, since Windows
// Services don't run from the exe's directory.
func agentDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// resolvePath resolves a path relative to the agent executable directory.
// If path is already absolute, returns it unchanged.
func resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(agentDir(), path)
}
