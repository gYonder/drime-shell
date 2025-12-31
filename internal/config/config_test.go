package config_test

import (
	"os"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoad_EnvVar(t *testing.T) {
	os.Setenv("DRIME_TOKEN", "env-token")
	defer os.Unsetenv("DRIME_TOKEN")

	cfg, err := config.Load()
	assert.NoError(t, err)
	assert.Equal(t, "env-token", cfg.Token)
}

func TestConfigPath(t *testing.T) {
	path, err := config.ConfigPath()
	assert.NoError(t, err)
	assert.Contains(t, path, ".drime-shell/config.yaml")
}
