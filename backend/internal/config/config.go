package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	Port     int      `yaml:"port"`
	Token    string   `yaml:"token"`
	Machines []string `yaml:"machines"`
	DataDir  string   `yaml:"data_dir"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Port:    8090,
		DataDir: "./data",
	}
}

// Load reads config from a YAML file. Missing file returns defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
