package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAML_MultiCall(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `
target: "localhost:50051"
proto: "service.proto"
concurrency: 10
total: 1000
timeout: "10s"
parallel: true
auth:
  type: "static"
  token: "my-jwt-token"
output:
  formats: ["cli", "html"]
  dir: "./reports"
calls:
  - call: "mypackage.MyService/DoWork"
    dynamic_fields:
      - field: "user_id"
        type: "uuid"
      - field: "amount"
        type: "int_range"
        min: 1
        max: 1000
  - call: "mypackage.MyService/GetStatus"
    dynamic_fields:
      - field: "status"
        type: "pool"
        values: ["active", "inactive", "pending"]
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromYAML(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromYAML() error = %v", err)
	}

	if cfg.Target != "localhost:50051" {
		t.Errorf("Target = %q, want %q", cfg.Target, "localhost:50051")
	}
	if cfg.Proto != "service.proto" {
		t.Errorf("Proto = %q, want %q", cfg.Proto, "service.proto")
	}
	if !cfg.Parallel {
		t.Error("Parallel should be true")
	}
	if len(cfg.Calls) != 2 {
		t.Fatalf("Calls count = %d, want 2", len(cfg.Calls))
	}
	if cfg.Calls[0].Call != "mypackage.MyService/DoWork" {
		t.Errorf("Calls[0].Call = %q", cfg.Calls[0].Call)
	}
	if len(cfg.Calls[0].DynamicFields) != 2 {
		t.Errorf("Calls[0].DynamicFields count = %d, want 2", len(cfg.Calls[0].DynamicFields))
	}
	if cfg.Calls[1].Call != "mypackage.MyService/GetStatus" {
		t.Errorf("Calls[1].Call = %q", cfg.Calls[1].Call)
	}
	if len(cfg.Calls[1].DynamicFields) != 1 {
		t.Errorf("Calls[1].DynamicFields count = %d, want 1", len(cfg.Calls[1].DynamicFields))
	}
}

func TestLoadFromYAML_FileNotFound(t *testing.T) {
	_, err := LoadFromYAML("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLoadFromYAML_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml}"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFromYAML(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestValidate_MissingTarget(t *testing.T) {
	cfg := &Config{
		Proto: "service.proto",
		Calls: []CallEntry{{Call: "pkg.Svc/Method"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing target")
	}
}

func TestValidate_MissingProto(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Calls:  []CallEntry{{Call: "pkg.Svc/Method"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing proto")
	}
}

func TestValidate_MissingCalls(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for empty calls list")
	}
}

func TestValidate_EmptyCallInList(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls:  []CallEntry{{Call: ""}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for empty call string")
	}
}

func TestValidate_ValidMinimal(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls:  []CallEntry{{Call: "pkg.Svc/Method"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_ValidMultipleCalls(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls: []CallEntry{
			{Call: "pkg.Svc/MethodA"},
			{Call: "pkg.Svc/MethodB"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_InvalidAuthType(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls:  []CallEntry{{Call: "pkg.Svc/Method"}},
		Auth:   AuthConfig{Type: "kerberos"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid auth type")
	}
}

func TestValidate_OAuthMissingEndpoint(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls:  []CallEntry{{Call: "pkg.Svc/Method"}},
		Auth:   AuthConfig{Type: "oauth", ClientID: "my-client"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for OAuth missing token_url")
	}
}

func TestValidate_InvalidDynamicFieldTypeInCall(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls: []CallEntry{
			{
				Call: "pkg.Svc/Method",
				DynamicFields: []DynamicFieldConfig{
					{Field: "name", Type: "unsupported_type"},
				},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid dynamic field type")
	}
}

func TestMergeFlags_CallOverrideSingleEntry(t *testing.T) {
	base := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls: []CallEntry{
			{Call: "pkg.Svc/MethodA"},
			{Call: "pkg.Svc/MethodB"},
		},
		Concurrency: 10,
		Total:       1000,
	}

	overrides := &FlagOverrides{
		Target:      "remote:50051",
		Concurrency: 50,
		Call:        "pkg.Svc/MethodA",
	}

	merged := MergeFlags(base, overrides)

	if merged.Target != "remote:50051" {
		t.Errorf("merged Target = %q, want remote:50051", merged.Target)
	}
	if merged.Concurrency != 50 {
		t.Errorf("merged Concurrency = %d, want 50", merged.Concurrency)
	}
	// --call override should reduce Calls to single entry
	if len(merged.Calls) != 1 {
		t.Fatalf("expected 1 call after --call override, got %d", len(merged.Calls))
	}
	if merged.Calls[0].Call != "pkg.Svc/MethodA" {
		t.Errorf("merged Calls[0].Call = %q, want pkg.Svc/MethodA", merged.Calls[0].Call)
	}
}

func TestMergeFlags_NoCallOverridePreservesAll(t *testing.T) {
	base := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls: []CallEntry{
			{Call: "pkg.Svc/MethodA"},
			{Call: "pkg.Svc/MethodB"},
		},
	}

	overrides := &FlagOverrides{Concurrency: 20}
	merged := MergeFlags(base, overrides)

	if len(merged.Calls) != 2 {
		t.Errorf("expected 2 calls preserved, got %d", len(merged.Calls))
	}
}

func TestMergeFlags_ParallelOverride(t *testing.T) {
	base := &Config{
		Target:   "localhost:50051",
		Proto:    "service.proto",
		Calls:    []CallEntry{{Call: "pkg.Svc/Method"}},
		Parallel: false,
	}

	overrides := &FlagOverrides{Parallel: true}
	merged := MergeFlags(base, overrides)

	if !merged.Parallel {
		t.Error("expected Parallel=true after flag override")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Calls:  []CallEntry{{Call: "pkg.Svc/Method"}},
	}

	ApplyDefaults(cfg)

	if cfg.Concurrency == 0 {
		t.Error("expected default concurrency to be set")
	}
	if cfg.Timeout == "" {
		t.Error("expected default timeout to be set")
	}
	if cfg.Output.Dir == "" {
		t.Error("expected default output dir to be set")
	}
}
