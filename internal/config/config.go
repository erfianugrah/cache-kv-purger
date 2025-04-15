package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultAPIEndpoint is the default Cloudflare API endpoint
	DefaultAPIEndpoint = "https://api.cloudflare.com/client/v4"

	// Environment variables for configuration
	EnvZoneID               = "CLOUDFLARE_ZONE_ID"
	EnvAccountID            = "CLOUDFLARE_ACCOUNT_ID"
	EnvCacheConcurrency     = "CLOUDFLARE_CACHE_CONCURRENCY"
	EnvMultiZoneConcurrency = "CLOUDFLARE_MULTI_ZONE_CONCURRENCY"

	// Default concurrency values for Enterprise tier
	DefaultCacheConcurrency     = 50 // Enterprise tier allows 50 requests per second
	DefaultMaxCacheConcurrency  = 50 // Enterprise tier cap
	DefaultMultiZoneConcurrency = 10 // Increased for Enterprise performance

	// Default batch size per API limits
	DefaultBatchSize = 100 // Maximum items per API request (Cloudflare limit)
)

// Config holds the application configuration
type Config struct {
	APIEndpoint          string `json:"api_endpoint"`
	DefaultZone          string `json:"default_zone,omitempty"`
	AccountID            string `json:"account_id,omitempty"`
	CacheConcurrency     int    `json:"cache_concurrency,omitempty"`
	MultiZoneConcurrency int    `json:"multi_zone_concurrency,omitempty"`
	
	// Runtime configuration values (not persisted)
	runtimeValues map[string]string
}

// New creates a Config with default values
func New() *Config {
	return &Config{
		APIEndpoint:          DefaultAPIEndpoint,
		CacheConcurrency:     DefaultCacheConcurrency,
		MultiZoneConcurrency: DefaultMultiZoneConcurrency,
		runtimeValues:        make(map[string]string),
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

	// Check environment variables for concurrency settings
	if envCacheConcurrency := os.Getenv(EnvCacheConcurrency); envCacheConcurrency != "" {
		var concurrency int
		if _, err := fmt.Sscanf(envCacheConcurrency, "%d", &concurrency); err == nil && concurrency > 0 {
			cfg.CacheConcurrency = concurrency
		}
	}

	if envMultiZoneConcurrency := os.Getenv(EnvMultiZoneConcurrency); envMultiZoneConcurrency != "" {
		var concurrency int
		if _, err := fmt.Sscanf(envMultiZoneConcurrency, "%d", &concurrency); err == nil && concurrency > 0 {
			cfg.MultiZoneConcurrency = concurrency
		}
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

// GetCacheConcurrency returns the cache concurrency setting from the config
func (c *Config) GetCacheConcurrency() int {
	// First check environment variable
	if envConcurrency := os.Getenv(EnvCacheConcurrency); envConcurrency != "" {
		var concurrency int
		if _, err := fmt.Sscanf(envConcurrency, "%d", &concurrency); err == nil && concurrency > 0 {
			return concurrency
		}
	}

	// Then use config value, with default as fallback
	if c.CacheConcurrency > 0 {
		return c.CacheConcurrency
	}
	return DefaultCacheConcurrency
}

// GetMultiZoneConcurrency returns the multi-zone concurrency setting from the config
func (c *Config) GetMultiZoneConcurrency() int {
	// First check environment variable
	if envConcurrency := os.Getenv(EnvMultiZoneConcurrency); envConcurrency != "" {
		var concurrency int
		if _, err := fmt.Sscanf(envConcurrency, "%d", &concurrency); err == nil && concurrency > 0 {
			return concurrency
		}
	}

	// Then use config value, with default as fallback
	if c.MultiZoneConcurrency > 0 {
		return c.MultiZoneConcurrency
	}
	return DefaultMultiZoneConcurrency
}

// fileExists checks if a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// SetValue sets a runtime configuration value
func (c *Config) SetValue(key, value string) {
	if c.runtimeValues == nil {
		c.runtimeValues = make(map[string]string)
	}
	c.runtimeValues[key] = value
}

// GetValue gets a runtime configuration value
func (c *Config) GetValue(key string) string {
	if c.runtimeValues == nil {
		return ""
	}
	return c.runtimeValues[key]
}

// IsVerbose returns true if verbose output is enabled
func (c *Config) IsVerbose() bool {
	return c.GetValue("verbose") == "true"
}

// IsDebug returns true if debug output is enabled
func (c *Config) IsDebug() bool {
	return c.GetValue("debug") == "true"
}
