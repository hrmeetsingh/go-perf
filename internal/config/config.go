package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Target      string            `yaml:"target"`
	Proto       string            `yaml:"proto"`
	Concurrency int               `yaml:"concurrency"`
	Total       int               `yaml:"total"`
	Duration    string            `yaml:"duration"`
	Connections int               `yaml:"connections"`
	Timeout     string            `yaml:"timeout"`
	Metadata    map[string]string `yaml:"metadata"`
	Auth        AuthConfig        `yaml:"auth"`
	Output      OutputConfig      `yaml:"output"`
	Stream      StreamConfig      `yaml:"stream"`
	Parallel    bool              `yaml:"parallel"`
	// Calls is the list of RPC calls to benchmark, each with its own dynamic_fields.
	Calls []CallEntry `yaml:"calls"`
}

// CallEntry represents a single RPC call within a multi-call config.
type CallEntry struct {
	Call          string               `yaml:"call"`
	DynamicFields []DynamicFieldConfig `yaml:"dynamic_fields"`
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
	// Call, when set, collapses Calls to a single-entry list.
	Call        string
	Concurrency int
	Total       int
	Duration    string
	Connections int
	Timeout     string
	Parallel    bool
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
	if len(c.Calls) == 0 {
		return fmt.Errorf("at least one call entry is required under 'calls:'")
	}
	for i, ce := range c.Calls {
		if ce.Call == "" {
			return fmt.Errorf("calls[%d]: call name is required", i)
		}
		for j, df := range ce.DynamicFields {
			if !validDynamicFieldTypes[df.Type] {
				return fmt.Errorf("calls[%d].dynamic_fields[%d]: invalid type %q", i, j, df.Type)
			}
		}
	}
	if !validAuthTypes[c.Auth.Type] {
		return fmt.Errorf("invalid auth type %q: must be static, oauth, or empty", c.Auth.Type)
	}
	if c.Auth.Type == "oauth" && c.Auth.TokenURL == "" {
		return fmt.Errorf("auth.token_url is required when auth type is oauth")
	}
	return nil
}

func MergeFlags(base *Config, overrides *FlagOverrides) *Config {
	merged := *base
	// Deep copy Calls slice so we don't mutate base.
	merged.Calls = make([]CallEntry, len(base.Calls))
	copy(merged.Calls, base.Calls)

	if overrides.Target != "" {
		merged.Target = overrides.Target
	}
	if overrides.Proto != "" {
		merged.Proto = overrides.Proto
	}
	// --call flag overrides Calls to a single-entry list.
	if overrides.Call != "" {
		merged.Calls = []CallEntry{{Call: overrides.Call}}
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
	if overrides.Parallel {
		merged.Parallel = true
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
