package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConsoleNoError(t *testing.T) {
	l := NewConsole()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	l.Close()
}

func TestNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := New(path, false)
	if err != nil {
		t.Fatalf("New(%q) unexpected error: %v", path, err)
	}
	defer l.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("log file was not created")
	}
}

func TestLogLevels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := New(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Info("info message")
	l.Warn("warn message: %s", "details")
	l.Error("error message")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "[INFO] info message") {
		t.Errorf("missing INFO line, got:\n%s", content)
	}
	if !strings.Contains(content, "[WARN] warn message: details") {
		t.Errorf("missing WARN line, got:\n%s", content)
	}
	if !strings.Contains(content, "[ERROR] error message") {
		t.Errorf("missing ERROR line, got:\n%s", content)
	}
}
