package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Aliases           map[string]string `yaml:"aliases,omitempty"`
	Theme             string            `yaml:"theme"`
	Token             string            `yaml:"token"`
	APIURL            string            `yaml:"api_url"`
	HistorySize       int               `yaml:"history_size"`
	MaxMemoryBufferMB int               `yaml:"max_memory_buffer_mb"`
}

const DefaultMaxMemoryBufferMB = 100 // 100MB

func Default() *Config {
	return &Config{
		Theme:             "auto",
		APIURL:            "https://app.drime.cloud/api/v1",
		HistorySize:       1000,
		MaxMemoryBufferMB: DefaultMaxMemoryBufferMB,
		Aliases:           make(map[string]string),
	}
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".drime-shell"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func HistoryPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history"), nil
}

func Load() (*Config, error) {
	cfg := Default()

	// 1. Load from file
	path, err := ConfigPath()
	if err == nil {
		f, err := os.Open(path)
		if err == nil {
			defer f.Close()
			if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// 2. Override from Env
	if token := os.Getenv("DRIME_TOKEN"); token != "" {
		cfg.Token = token
	}

	return cfg, nil
}

// Save writes the config to ~/.drime-shell/config.yaml
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// Write with secure permissions (0600 = owner read/write only)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
