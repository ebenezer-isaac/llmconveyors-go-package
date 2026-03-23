package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds all CLI configuration values.
type Config struct {
	APIKey     string        `mapstructure:"api_key"`
	BaseURL    string        `mapstructure:"base_url"`
	Output     string        `mapstructure:"output"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries int           `mapstructure:"max_retries"`
	Debug      bool          `mapstructure:"debug"`
	NoColor    bool          `mapstructure:"no_color"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		BaseURL:    "https://api.llmconveyors.com/api/v1",
		Output:     "text",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Debug:      false,
		NoColor:    false,
	}
}

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

// Load initializes viper with env/config/flag precedence and returns the Config.
// Priority: flag > env var > config file > default
func Load(configPath string) (Config, error) {
	v := viper.New()

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("api_key", defaults.APIKey)
	v.SetDefault("base_url", defaults.BaseURL)
	v.SetDefault("output", defaults.Output)
	v.SetDefault("timeout", defaults.Timeout)
	v.SetDefault("max_retries", defaults.MaxRetries)
	v.SetDefault("debug", defaults.Debug)
	v.SetDefault("no_color", defaults.NoColor)

	// Env var binding: LLMC_API_KEY, LLMC_BASE_URL, etc.
	v.SetEnvPrefix("LLMC")
	v.AutomaticEnv()

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		dir := DefaultConfigDir()
		if dir != "" {
			v.AddConfigPath(dir)
			v.SetConfigName("config")
			v.SetConfigType("yaml")
		}
	}

	if err := v.ReadInConfig(); err != nil {
		// Config file not found is fine — everything else is an error
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if !os.IsNotExist(err) {
				return Config{}, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// EnsureConfigDir creates ~/.llmconveyors/ if it doesn't exist.
func EnsureConfigDir() (string, error) {
	dir := DefaultConfigDir()
	if dir == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	return dir, nil
}

// WriteConfigFile writes a config map to ~/.llmconveyors/config.yaml.
func WriteConfigFile(path string, values map[string]interface{}) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	for k, val := range values {
		v.Set(k, val)
	}

	return v.WriteConfigAs(path)
}
