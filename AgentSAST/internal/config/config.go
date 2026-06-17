// Package config loads AgentSAST settings from a YAML file whose values are
// interpolated from the environment (and optional .env files).
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	MinIO  MinIOConfig  `yaml:"minio"`
	LLM    LLMConfig    `yaml:"llm"`
}

type ServerConfig struct {
	Port     string `yaml:"port"`
	WorkRoot string `yaml:"work_root"`
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type MinIOConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	UseSSL    bool   `yaml:"use_ssl"`
}

type LLMConfig struct {
	BaseURL string `yaml:"llm_base_url"`
	APIKey  string `yaml:"llm_api_key"`
	Model   string `yaml:"llm_model"`
}

// Load reads .env files into the environment, then parses configPath with
// ${VAR} expansion and fills in defaults.
func Load(configPath string) (Config, error) {
	// Root infra creds first, then service-local overrides.
	loadDotenv("../.env")
	loadDotenv(".env")

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}
	expanded := os.Expand(string(raw), func(k string) string { return os.Getenv(k) })

	var c Config
	if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
		return Config{}, err
	}
	c.applyDefaults()
	return c, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Port == "" {
		c.Server.Port = "8001"
	}
	if c.Server.WorkRoot == "" {
		c.Server.WorkRoot = filepath.Join(os.TempDir(), "agentsast")
	}
	if c.MinIO.Endpoint == "" {
		c.MinIO.Endpoint = "127.0.0.1:9000"
	}
	if c.MinIO.Bucket == "" {
		c.MinIO.Bucket = "agentsdast"
	}
}

// loadDotenv loads KEY=VALUE pairs from path (if it exists). Existing env vars
// win, so real environment overrides file values.
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
