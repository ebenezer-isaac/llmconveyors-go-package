package config

import (
	"os"
	"path/filepath"
)

// DefaultConfigDir returns ~/.llmconveyors/
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".llmconveyors")
}

// DefaultConfigPath returns ~/.llmconveyors/config.yaml
func DefaultConfigPath() string {
	dir := DefaultConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}
