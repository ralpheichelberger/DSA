package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DevMode(t *testing.T) {
	t.Setenv("DEV_MODE", "true")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("MINEA_EMAIL", "")
	t.Setenv("MINEA_PASSWORD", "")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.DevMode)
}

func TestLoad_ProdMode(t *testing.T) {
	t.Setenv("DEV_MODE", "false")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("MINEA_EMAIL", "owner@example.com")
	t.Setenv("MINEA_PASSWORD", "secret")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.False(t, cfg.DevMode)
	assert.Error(t, cfg.Validate())
}

func TestDefaults(t *testing.T) {
	t.Setenv("DEV_MODE", "true")
	t.Setenv("DB_PATH", "")
	t.Setenv("PORT", "")
	t.Setenv("AGENT_DISCOVERY_NICHES", "")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "./data/agent.db", cfg.DatabasePath)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, []string{"tech", "pets"}, cfg.DiscoveryNiches)
}

func TestLoad_DiscoveryNichesCustom(t *testing.T) {
	t.Setenv("DEV_MODE", "true")
	t.Setenv("AGENT_DISCOVERY_NICHES", " pets , home ")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, []string{"pets", "home"}, cfg.DiscoveryNiches)
}
