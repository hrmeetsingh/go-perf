package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Target        string              `yaml:"target"`
	Proto         string              `yaml:"proto"`
	Call          string              `yaml:"call"`
	Concurrency   int                 `yaml:"concurrency"`
	Total         int                 `yaml:"total"`
	Duration      string              `yaml:"duration"`
	Connections   int                 `yaml:"connections"`
	Timeout       string              `yaml:"timeout"`
	Metadata      map[string]string   `yaml:"metadata"`
	Auth          AuthConfig          `yaml:"auth"`
	DynamicFields []DynamicFieldConfig `yaml:"dynamic_fields"`
	Output        OutputConfig        `yaml:"output"`
	Stream        StreamConfig        `yaml:"stream"`
}

type AuthConfig struct {
	Type         string `yaml:"type"`
	Token        string `yaml:"token"`
	TokenURL     string `yaml:"token_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type DynamicFieldConfig struct {
	Field  string        `yaml:"field"`
	Type   string        `yaml:"type"`
	Min    int64         `yaml:"min,omitempty"`
	Max    int64         `yaml:"max,omitempty"`
	Length int           `yaml:"length,omitempty"`
	Values []interface{} `yaml:"values,omitempty"`
}

type OutputConfig struct {
	Formats []string `yaml:"formats"`
	Dir     string   `yaml:"dir"`
}

type StreamConfig struct {
	SendRate    int `yaml:"send_rate"`
	StreamCount int `yaml:"stream_count"`
}

type FlagOverrides struct {
	Target      string
	Proto       string
	Call        string
	Concurrency int
	Total       int
	Duration    string
	Connections int
	Timeout     string
}

var validAuthTypes = map[string]bool{
	"":       true,
	"static": true,
	"oauth":  true,
}

var validDynamicFieldTypes = map[string]bool{
	"uuid":      true,
	"int_range": true,
	"pool":      true,
	"string":    true,
	"timestamp": true,
}

func LoadFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Target == "" {
		return fmt.Errorf("target is required")
	}
	if c.Proto == "" {
		return fmt.Errorf("proto is required")
	}
	if c.Call == "" {
		return fmt.Errorf("call is required")
	}
	if !validAuthTypes[c.Auth.Type] {
		return fmt.Errorf("invalid auth type %q: must be static, oauth, or empty", c.Auth.Type)
	}
	if c.Auth.Type == "oauth" && c.Auth.TokenURL == "" {
		return fmt.Errorf("auth.token_url is required when auth type is oauth")
	}
	for i, df := range c.DynamicFields {
		if !validDynamicFieldTypes[df.Type] {
			return fmt.Errorf("dynamic_fields[%d]: invalid type %q", i, df.Type)
		}
	}
	return nil
}

func MergeFlags(base *Config, overrides *FlagOverrides) *Config {
	merged := *base
	if overrides.Target != "" {
		merged.Target = overrides.Target
	}
	if overrides.Proto != "" {
		merged.Proto = overrides.Proto
	}
	if overrides.Call != "" {
		merged.Call = overrides.Call
	}
	if overrides.Concurrency != 0 {
		merged.Concurrency = overrides.Concurrency
	}
	if overrides.Total != 0 {
		merged.Total = overrides.Total
	}
	if overrides.Duration != "" {
		merged.Duration = overrides.Duration
	}
	if overrides.Connections != 0 {
		merged.Connections = overrides.Connections
	}
	if overrides.Timeout != "" {
		merged.Timeout = overrides.Timeout
	}
	return &merged
}

func ApplyDefaults(cfg *Config) {
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 50
	}
	if cfg.Total == 0 {
		cfg.Total = 200
	}
	if cfg.Connections == 0 {
		cfg.Connections = 1
	}
	if cfg.Timeout == "" {
		cfg.Timeout = "20s"
	}
	if cfg.Output.Dir == "" {
		cfg.Output.Dir = "./reports"
	}
	if len(cfg.Output.Formats) == 0 {
		cfg.Output.Formats = []string{"cli"}
	}
}
