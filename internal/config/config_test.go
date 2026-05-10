package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `
target: "localhost:50051"
proto: "service.proto"
call: "mypackage.MyService/DoWork"
concurrency: 10
total: 1000
duration: "30s"
connections: 5
timeout: "10s"
metadata:
  x-request-id: "test"
auth:
  type: "static"
  token: "my-jwt-token"
dynamic_fields:
  - field: "user_id"
    type: "uuid"
  - field: "amount"
    type: "int_range"
    min: 1
    max: 1000
  - field: "status"
    type: "pool"
    values: ["active", "inactive", "pending"]
output:
  formats: ["cli", "html", "json", "junit"]
  dir: "./reports"
stream:
  send_rate: 100
  stream_count: 5
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
	if cfg.Call != "mypackage.MyService/DoWork" {
		t.Errorf("Call = %q, want %q", cfg.Call, "mypackage.MyService/DoWork")
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, 10)
	}
	if cfg.Total != 1000 {
		t.Errorf("Total = %d, want %d", cfg.Total, 1000)
	}
	if cfg.Duration != "30s" {
		t.Errorf("Duration = %q, want %q", cfg.Duration, "30s")
	}
	if cfg.Connections != 5 {
		t.Errorf("Connections = %d, want %d", cfg.Connections, 5)
	}
	if cfg.Timeout != "10s" {
		t.Errorf("Timeout = %q, want %q", cfg.Timeout, "10s")
	}
	if cfg.Auth.Type != "static" {
		t.Errorf("Auth.Type = %q, want %q", cfg.Auth.Type, "static")
	}
	if cfg.Auth.Token != "my-jwt-token" {
		t.Errorf("Auth.Token = %q, want %q", cfg.Auth.Token, "my-jwt-token")
	}
	if len(cfg.DynamicFields) != 3 {
		t.Errorf("DynamicFields count = %d, want 3", len(cfg.DynamicFields))
	}
	if cfg.DynamicFields[0].Field != "user_id" || cfg.DynamicFields[0].Type != "uuid" {
		t.Errorf("DynamicFields[0] = %+v, want user_id/uuid", cfg.DynamicFields[0])
	}
	if len(cfg.Output.Formats) != 4 {
		t.Errorf("Output.Formats count = %d, want 4", len(cfg.Output.Formats))
	}
	if cfg.Stream.SendRate != 100 {
		t.Errorf("Stream.SendRate = %d, want 100", cfg.Stream.SendRate)
	}
	if cfg.Stream.StreamCount != 5 {
		t.Errorf("Stream.StreamCount = %d, want 5", cfg.Stream.StreamCount)
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
		Call:  "pkg.Svc/Method",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing target")
	}
}

func TestValidate_MissingProto(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Call:   "pkg.Svc/Method",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing proto")
	}
}

func TestValidate_MissingCall(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing call")
	}
}

func TestValidate_ValidMinimal(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Call:   "pkg.Svc/Method",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_InvalidAuthType(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Call:   "pkg.Svc/Method",
		Auth: AuthConfig{
			Type: "kerberos",
		},
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
		Call:   "pkg.Svc/Method",
		Auth: AuthConfig{
			Type:     "oauth",
			ClientID: "my-client",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for OAuth missing token_url")
	}
}

func TestValidate_InvalidDynamicFieldType(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Call:   "pkg.Svc/Method",
		DynamicFields: []DynamicFieldConfig{
			{Field: "name", Type: "unsupported_type"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid dynamic field type")
	}
}

func TestMergeFlags(t *testing.T) {
	base := &Config{
		Target:      "localhost:50051",
		Proto:       "service.proto",
		Call:        "pkg.Svc/Method",
		Concurrency: 10,
		Total:       1000,
	}

	overrides := &FlagOverrides{
		Target:      "remote:50051",
		Concurrency: 50,
	}

	merged := MergeFlags(base, overrides)

	if merged.Target != "remote:50051" {
		t.Errorf("merged Target = %q, want %q", merged.Target, "remote:50051")
	}
	if merged.Proto != "service.proto" {
		t.Errorf("merged Proto = %q, want %q", merged.Proto, "service.proto")
	}
	if merged.Concurrency != 50 {
		t.Errorf("merged Concurrency = %d, want %d", merged.Concurrency, 50)
	}
	if merged.Total != 1000 {
		t.Errorf("merged Total should remain %d, got %d", 1000, merged.Total)
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Target: "localhost:50051",
		Proto:  "service.proto",
		Call:   "pkg.Svc/Method",
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
