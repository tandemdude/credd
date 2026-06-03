package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server      ServerConfig      `toml:"server"`
	OnePassword OnePasswordConfig `toml:"1password"`
}

type ServerConfig struct {
	Addr string `toml:"addr"`
}

type OnePasswordConfig struct {
	Account string `toml:"account"`
}

// DefaultDir resolves the credd home directory: $CREDD_HOME if set, else
// '~/.credd'.
func DefaultDir() (string, error) {
	if h := os.Getenv("CREDD_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".credd"), nil
}

func DefaultConfigPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func DataDBPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "data.db")
}

// Load parses the TOML config file at path. The file must exist.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Validate returns nil if the config file is absent or parses cleanly.
func Validate(path string) error {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat config %s: %w", path, err)
	}
	if _, err := Load(path); err != nil {
		return err
	}
	return nil
}

func Write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
