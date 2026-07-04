// Package config loads tradectl's user configuration from
// ~/.tradectl/config.yaml, applying sane defaults and creating the file on
// first run if it does not exist.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config mirrors ~/.tradectl/config.yaml. Unknown keys in an existing file are
// ignored, so older configs keep working.
type Config struct {
	// Root directory for the SQLite DB and screenshots. A leading "~/" is
	// expanded to the user's home directory. The default is a fixed location
	// under the home dir so tradectl behaves the same no matter which
	// directory it is launched from (it is installed as a global command).
	DataDir string `yaml:"data_dir"`
}

// Defaults returns a Config populated with the default values.
func Defaults() Config {
	return Config{
		DataDir: "~/.tradectl/data",
	}
}

// expandHome resolves a leading "~" or "~/" in path to the home directory.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

// Path returns the absolute path to the config file (~/.tradectl/config.yaml).
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".tradectl", "config.yaml"), nil
}

// Load reads the config file, applying defaults for any missing fields. If the
// file does not exist it is created with the default values and createdPath is
// returned so the caller can inform the user.
func Load() (cfg Config, createdPath string, err error) {
	cfg = Defaults()

	path, err := Path()
	if err != nil {
		return cfg, "", err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Persist the portable "~/" form; return the expanded path for use.
		if werr := write(path, cfg); werr != nil {
			return cfg, "", werr
		}
		if cfg.DataDir, err = expandHome(cfg.DataDir); err != nil {
			return cfg, "", err
		}
		return cfg, path, nil
	}
	if err != nil {
		return cfg, "", fmt.Errorf("reading %s: %w", path, err)
	}

	// Unmarshal over the defaults so unspecified keys keep their default value.
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, "", fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.DataDir, err = expandHome(cfg.DataDir); err != nil {
		return cfg, "", err
	}
	return cfg, "", nil
}

func write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	header := "# tradectl configuration\n"
	if err := os.WriteFile(path, append([]byte(header), out...), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
