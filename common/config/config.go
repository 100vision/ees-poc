package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the agent configuration file (config.json).
type Config struct {
	// PipeName is the named pipe path, e.g. `\\.\pipe\ees`.
	PipeName string `json:"PipeName"`

	// Whitelist is the path to the whitelist JSON file.
	Whitelist string `json:"Whitelist"`

	// LogPath is the path to the agent log file.
	LogPath string `json:"LogPath"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		PipeName:  `\\.\pipe\ees`,
		Whitelist: filepath.Join("config", "whitelist.json"),
		LogPath:   filepath.Join("logs", "agent.log"),
	}
}

// Load reads a JSON config file from path and returns a validated Config.
// If path is empty, returns DefaultConfig.
func Load(path string) (*Config, error) {
	if path == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validation: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for invalid or missing values.
func (c *Config) Validate() error {
	if c.PipeName == "" {
		return fmt.Errorf("PipeName must not be empty")
	}
	if c.Whitelist == "" {
		return fmt.Errorf("Whitelist path must not be empty")
	}
	if c.LogPath == "" {
		return fmt.Errorf("LogPath must not be empty")
	}
	return nil
}

// Save writes the config to a JSON file at the given path.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config: create dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config: write file: %w", err)
	}
	return nil
}
