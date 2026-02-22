package config

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all runtime configuration for Auspex.
type Config struct {
	Port            int       `yaml:"port"`
	DBPath          string    `yaml:"db_path"`
	RefreshInterval int       `yaml:"refresh_interval"` // minutes
	ESI             ESIConfig `yaml:"esi"`
}

// ESIConfig holds EVE SSO / ESI credentials.
type ESIConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	CallbackURL  string `yaml:"callback_url"`
}

// Load reads the config file path from CLI flags and delegates to loadFromFile.
func Load() (*Config, error) {
	configPath := flag.String("config", "auspex.yaml", "path to config file")
	flag.Parse()
	return loadFromFile(*configPath)
}

func loadFromFile(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Port:            8080,
		DBPath:          "auspex.db",
		RefreshInterval: 10,
	}
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0, got %d", c.RefreshInterval)
	}
	if c.ESI.ClientID == "" {
		return fmt.Errorf("esi.client_id is required")
	}
	if c.ESI.ClientSecret == "" {
		return fmt.Errorf("esi.client_secret is required")
	}
	if c.ESI.CallbackURL == "" {
		return fmt.Errorf("esi.callback_url is required")
	}
	return nil
}
