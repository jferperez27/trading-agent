// Package config loads tradectl's user configuration from
// ~/.tradectl/config.yaml, applying sane defaults and creating the file on
// first run if it does not exist.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config mirrors ~/.tradectl/config.yaml (see Documentation/00-MASTER.md).
type Config struct {
	// Name of the environment variable that holds the Anthropic API key.
	// The key itself is never stored in config or on disk.
	AnthropicAPIKeyEnv string `yaml:"anthropic_api_key_env"`
	// Default model for per-trade analysis (lightweight check).
	DefaultModelTrade string `yaml:"default_model_trade"`
	// Default model for per-session critique.
	DefaultModelSession string `yaml:"default_model_session"`
	// Number of prior session summaries to feed into a new session analysis.
	// Consumed in Sprint 2; carried here so config is stable across sprints.
	LongitudinalContextCount int `yaml:"longitudinal_context_count"`
	// Monthly spend threshold (used in Sprint 3).
	MonthlyCostAlertThresholdUSD float64 `yaml:"monthly_cost_alert_threshold_usd"`
	// Root directory for the SQLite DB and screenshots.
	DataDir string `yaml:"data_dir"`
}

// Defaults returns a Config populated with the documented default values.
func Defaults() Config {
	return Config{
		AnthropicAPIKeyEnv:           "ANTHROPIC_API_KEY",
		DefaultModelTrade:            "claude-haiku-4-5-20251001",
		DefaultModelSession:          "claude-sonnet-4-6",
		LongitudinalContextCount:     3,
		MonthlyCostAlertThresholdUSD: 10.00,
		DataDir:                      "./data",
	}
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
		if werr := write(path, cfg); werr != nil {
			return cfg, "", werr
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
	return cfg, "", nil
}

// APIKey resolves the Anthropic API key from the configured environment
// variable. It returns a clear error if the variable is unset or empty so that
// analyze fails fast rather than hanging or panicking.
func (c Config) APIKey() (string, error) {
	envName := c.AnthropicAPIKeyEnv
	if envName == "" {
		envName = "ANTHROPIC_API_KEY"
	}
	key := os.Getenv(envName)
	if key == "" {
		return "", fmt.Errorf("API key not found: environment variable %s is unset or empty", envName)
	}
	return key, nil
}

func write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	header := "# tradectl configuration\n" +
		"# The API key itself is read from the environment variable named below;\n" +
		"# it is never stored here.\n"
	if err := os.WriteFile(path, append([]byte(header), out...), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
