// Package config handles loading and persisting application configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds every user-configurable setting for MinerGate.
type Config struct {
	Language        string `json:"language"`
	AutoRefresh     bool   `json:"auto_refresh"`
	RefreshRate     int    `json:"refresh_rate"`      // seconds
	Theme           string `json:"theme"`             // "dark" | "light"
	APIEndpoint     string `json:"api_endpoint"`      // legacy field, kept for compat
	APITargetDevice string `json:"api_target_device"` // legacy field
	FRPEnabled      bool   `json:"frp_enabled"`
	GoASICEnabled   bool   `json:"goasic_enabled"`
	UpdateAutoCheck bool   `json:"update_auto_check"`

	// Miner targets (host:port pairs added via the UI)
	Miners []MinerTarget `json:"miners"`

	// Data storage
	DataDir string `json:"data_dir"`
}

// MinerTarget is one device the user wants to monitor.
type MinerTarget struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"` // HTTP port (default 80)
	APIPort int    `json:"api_port"` // cgminer API port (default 4028)
	Enabled bool   `json:"enabled"`
}

// Default returns sensible factory defaults.
func Default() *Config {
	dataDir, _ := os.UserConfigDir()
	dataDir = filepath.Join(dataDir, "minergate", "data")

	return &Config{
		Language:        "en",
		AutoRefresh:     true,
		RefreshRate:     15,
		Theme:           "dark",
		FRPEnabled:      false,
		GoASICEnabled:   true,
		UpdateAutoCheck: true,
		DataDir:         dataDir,
		Miners: []MinerTarget{
			{
				Name:    "Local Simulator",
				Host:    "localhost",
				Port:    8081,
				APIPort: 4028,
				Enabled: true,
			},
		},
	}
}

// configPath returns the path to the JSON config file.
func configPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "minergate", "config.json"), nil
}

// Load reads the config file; falls back to Default() if it doesn't exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save persists the current config to disk.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}

// CSVPath returns the full path to the hashrate history CSV for the given key.
// key is typically the miner host or "total".
func (c *Config) CSVPath(key string) string {
	safe := ""
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			safe += string(r)
		} else {
			safe += "_"
		}
	}
	return filepath.Join(c.DataDir, safe+"_hashrate.csv")
}
