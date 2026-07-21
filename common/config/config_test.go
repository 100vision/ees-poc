package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.PipeName != `\\.\pipe\ees` {
		t.Errorf("expected default PipeName, got %q", cfg.PipeName)
	}
	if cfg.Whitelist == "" {
		t.Error("expected non-empty Whitelist")
	}
	if cfg.LogPath == "" {
		t.Error("expected non-empty LogPath")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{PipeName: `\\.\pipe\ees`, Whitelist: "wl.json", LogPath: "log.txt"}, false},
		{"empty pipe", Config{PipeName: "", Whitelist: "wl.json", LogPath: "log.txt"}, true},
		{"empty whitelist", Config{PipeName: `\\.\pipe\ees`, Whitelist: "", LogPath: "log.txt"}, true},
		{"empty logpath", Config{PipeName: `\\.\pipe\ees`, Whitelist: "wl.json", LogPath: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") unexpected error: %v", err)
	}
	if cfg.PipeName != `\\.\pipe\ees` {
		t.Errorf("expected default PipeName, got %q", cfg.PipeName)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("nonexistent.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
