// Package config loads optional YAML configuration with environment overrides.
package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"agentdast/pkg/types"
)

// Config is the on-disk configuration file structure.
type Config struct {
	Scan ScanDefaults   `yaml:"scan"`
	AI   types.AIConfig `yaml:"ai"`
}

// ScanDefaults holds default scan settings overridable per-run by CLI flags.
type ScanDefaults struct {
	TargetBaseURL string            `yaml:"target_base_url"`
	Headers       map[string]string `yaml:"headers"`
	Plugins       []string          `yaml:"plugins"`
	Timeout       int               `yaml:"timeout"`
	Concurrency   int               `yaml:"concurrency"`
	OutputMode    string            `yaml:"output_mode"`
}

// Load reads a YAML config file. A missing path returns an empty config.
// Environment variables override file values for AI credentials.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("OPENAI_API_KEY"); v != "" && cfg.AI.APIKey == "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" && cfg.AI.BaseURL == "" {
		cfg.AI.BaseURL = v
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" && cfg.AI.Model == "" {
		cfg.AI.Model = v
	}
}
