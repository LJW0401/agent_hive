package config

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	Port     int      `yaml:"port"`
	Token    string   `yaml:"token"`
	Machines []string `yaml:"machines"`
	DataDir  string   `yaml:"data_dir"`
	User     string   `yaml:"user"`
	Shell    string   `yaml:"shell"`
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

	if cfg.User == "" || cfg.Shell == "" {
		if err := cfg.inferUserFromFile(path); err != nil {
			// Not fatal — fall back to current process user
		}
	}

	return cfg, nil
}

// inferUserFromFile detects the config file owner and fills in User/Shell if empty.
func (c *Config) inferUserFromFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("unsupported platform for file owner detection")
	}

	u, err := user.LookupId(strconv.Itoa(int(stat.Uid)))
	if err != nil {
		return err
	}

	if c.User == "" {
		c.User = u.Username
	}
	if c.Shell == "" {
		c.Shell = LookupUserShell(u.Username)
	}
	return nil
}

// LookupUserShell finds the default shell for a user from /etc/passwd.
func LookupUserShell(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return "/bin/bash"
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 7 && fields[0] == username {
			shell := strings.TrimSpace(fields[6])
			if shell != "" {
				return shell
			}
		}
	}
	return "/bin/bash"
}
