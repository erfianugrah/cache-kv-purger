package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	// DefaultAPIEndpoint is the default Cloudflare API endpoint
	DefaultAPIEndpoint = "https://api.cloudflare.com/client/v4"
	
	// Environment variables for configuration
	EnvZoneID    = "CLOUDFLARE_ZONE_ID"
	EnvAccountID = "CLOUDFLARE_ACCOUNT_ID"
)

// Config holds the application configuration
type Config struct {
	APIEndpoint string `json:"api_endpoint"`
	DefaultZone string `json:"default_zone,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
}

// New creates a Config with default values
func New() *Config {
	return &Config{
		APIEndpoint: DefaultAPIEndpoint,
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*Config, error) {
	if path == "" {
		// Try to find config in default locations
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return New(), nil // Return default config if we can't find home dir
		}

		// Check home directory first
		homePath := filepath.Join(homeDir, ".cache-kv-purger.json")
		if fileExists(homePath) {
			path = homePath
		}
	}

	var cfg *Config
	if path == "" || !fileExists(path) {
		cfg = New()
	} else {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		cfg = New()
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Always ensure API endpoint is set
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = DefaultAPIEndpoint
	}

	// Check environment variables for zone ID and account ID
	if envZoneID := os.Getenv(EnvZoneID); envZoneID != "" {
		cfg.DefaultZone = envZoneID
	}

	if envAccountID := os.Getenv(EnvAccountID); envAccountID != "" {
		cfg.AccountID = envAccountID
	}

	return cfg, nil
}

// SaveToFile saves the configuration to a JSON file
func (c *Config) SaveToFile(path string) error {
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errors.New("cannot determine home directory for config file")
		}
		path = filepath.Join(homeDir, ".cache-kv-purger.json")
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetZoneID returns the zone ID from the config, or an empty string if not set
func (c *Config) GetZoneID() string {
	// First check environment variable (highest priority)
	if envZoneID := os.Getenv(EnvZoneID); envZoneID != "" {
		return envZoneID
	}
	// Then use config value
	return c.DefaultZone
}

// GetAccountID returns the account ID from the config, or an empty string if not set
func (c *Config) GetAccountID() string {
	// First check environment variable (highest priority)
	if envAccountID := os.Getenv(EnvAccountID); envAccountID != "" {
		return envAccountID
	}
	// Then use config value
	return c.AccountID
}

// fileExists checks if a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}