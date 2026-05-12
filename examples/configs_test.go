package examples

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

type exampleCallEntry struct {
	Call string `yaml:"call"`
}

type exampleConfig struct {
	Target string             `yaml:"target"`
	Proto  string             `yaml:"proto"`
	Calls  []exampleCallEntry `yaml:"calls"`
}

func TestExampleConfigs_ParseCorrectly(t *testing.T) {
	configs := []string{
		"configs/quick.yaml",
		"configs/full.yaml",
		"configs/dynamic.yaml",
		"configs/stream.yaml",
	}

	for _, path := range configs {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var cfg exampleConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			if cfg.Target == "" {
				t.Errorf("%s: target is empty", path)
			}
			if cfg.Proto == "" {
				t.Errorf("%s: proto is empty", path)
			}
			if len(cfg.Calls) == 0 {
				t.Errorf("%s: calls list is empty", path)
			}
			for i, ce := range cfg.Calls {
				if ce.Call == "" {
					t.Errorf("%s: calls[%d].call is empty", path, i)
				}
			}
		})
	}
}

func TestExampleConfigs_ProtoFileReferenced(t *testing.T) {
	// All configs should reference a proto path that exists.
	// Configs use paths relative to the repo root; tests run from examples/,
	// so resolve via "../".
	configs := []string{
		"configs/quick.yaml",
		"configs/full.yaml",
		"configs/dynamic.yaml",
	}

	for _, path := range configs {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var cfg exampleConfig
			yaml.Unmarshal(data, &cfg)

			// Resolve from repo root (parent of examples/)
			resolved := "../" + cfg.Proto
			if _, err := os.Stat(resolved); os.IsNotExist(err) {
				t.Errorf("%s references proto %q which does not exist (checked %s)", path, cfg.Proto, resolved)
			}
		})
	}
}
