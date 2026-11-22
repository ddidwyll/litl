package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey    string            `yaml:"api_key"`
	AuthToken string            `yaml:"auth_token"`
	Actions   map[string]Action `yaml:"actions"`
	basePath  string
}

type Action struct {
	SystemPrompt string   `yaml:"system_prompt"`
	ContextFile  string   `yaml:"context_file"`
	Model        string   `yaml:"model"`
	Temperature  *float64 `yaml:"temperature,omitempty"`
	MaxTokens    *int     `yaml:"max_tokens,omitempty"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		configDir, _ := os.UserConfigDir()
		path = filepath.Join(configDir, "titl", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.basePath = filepath.Dir(path)

	// KEY ENV
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("ROUTER_API_KEY")
	}
	if cfg.AuthToken == "" {
		cfg.AuthToken = os.Getenv("LITL_AUTH_TOKEN")
	}

	return &cfg, nil
}

func (c *Config) LoadContext(filename string) (string, error) {
	path := filepath.Join(c.basePath, "contexts", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
