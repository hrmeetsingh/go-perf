package proto

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleProto = `
syntax = "proto3";

package testpkg;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
  rpc Chat (stream ChatMessage) returns (stream ChatMessage);
}

message HelloRequest {
  string name = 1;
  int32 age = 2;
  bool active = 3;
  double score = 4;
  Address address = 5;
}

message Address {
  string street = 1;
  string city = 2;
  int32 zip = 3;
}

message HelloReply {
  string message = 1;
}

message ChatMessage {
  string sender = 1;
  string text = 2;
  int64 timestamp = 3;
}
`

func writeProtoFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverProtoFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	files, err := DiscoverProtoFiles(protoPath)
	if err != nil {
		t.Fatalf("DiscoverProtoFiles() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != protoPath {
		t.Errorf("file = %q, want %q", files[0], protoPath)
	}
}

func TestDiscoverProtoFiles_Directory(t *testing.T) {
	dir := t.TempDir()
	writeProtoFile(t, dir, "a.proto", sampleProto)
	writeProtoFile(t, dir, "b.proto", sampleProto)
	writeProtoFile(t, dir, "not_proto.txt", "hello")

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeProtoFile(t, sub, "c.proto", sampleProto)

	files, err := DiscoverProtoFiles(dir)
	if err != nil {
		t.Fatalf("DiscoverProtoFiles() error = %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 proto files, got %d: %v", len(files), files)
	}
}

func TestDiscoverProtoFiles_NotFound(t *testing.T) {
	_, err := DiscoverProtoFiles("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestDiscoverProtoFiles_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	files, err := DiscoverProtoFiles(dir)
	if err != nil {
		t.Fatalf("DiscoverProtoFiles() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 proto files in empty dir, got %d", len(files))
	}
}

func TestParseProtoFile(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	descriptor, err := ParseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ParseProtoFile() error = %v", err)
	}
	if descriptor == nil {
		t.Fatal("expected non-nil descriptor")
	}
}

func TestParseProtoFile_InvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "bad.proto", "this is not valid proto syntax {{{")

	_, err := ParseProtoFile(protoPath)
	if err == nil {
		t.Error("expected error for invalid proto syntax")
	}
}

func TestListServices(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	descriptor, err := ParseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ParseProtoFile() error = %v", err)
	}

	services := ListServices(descriptor)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0] != "testpkg.Greeter" {
		t.Errorf("service = %q, want %q", services[0], "testpkg.Greeter")
	}
}

func TestListMethods(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	descriptor, err := ParseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ParseProtoFile() error = %v", err)
	}

	methods, err := ListMethods(descriptor, "testpkg.Greeter")
	if err != nil {
		t.Fatalf("ListMethods() error = %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}

	found := map[string]bool{}
	for _, m := range methods {
		found[m.Name] = true
	}
	if !found["SayHello"] {
		t.Error("expected SayHello method")
	}
	if !found["Chat"] {
		t.Error("expected Chat method")
	}
}

func TestListMethods_ServiceNotFound(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	descriptor, err := ParseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ParseProtoFile() error = %v", err)
	}

	_, err = ListMethods(descriptor, "nonexistent.Service")
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestGetMethodInfo(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	descriptor, err := ParseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ParseProtoFile() error = %v", err)
	}

	info, err := GetMethodInfo(descriptor, "testpkg.Greeter", "SayHello")
	if err != nil {
		t.Fatalf("GetMethodInfo() error = %v", err)
	}
	if info.IsClientStream {
		t.Error("SayHello should not be client streaming")
	}
	if info.IsServerStream {
		t.Error("SayHello should not be server streaming")
	}

	info, err = GetMethodInfo(descriptor, "testpkg.Greeter", "Chat")
	if err != nil {
		t.Fatalf("GetMethodInfo() error = %v", err)
	}
	if !info.IsClientStream {
		t.Error("Chat should be client streaming")
	}
	if !info.IsServerStream {
		t.Error("Chat should be server streaming")
	}
}

func TestGetInputFields(t *testing.T) {
	dir := t.TempDir()
	protoPath := writeProtoFile(t, dir, "service.proto", sampleProto)

	descriptor, err := ParseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ParseProtoFile() error = %v", err)
	}

	fields, err := GetInputFields(descriptor, "testpkg.Greeter", "SayHello")
	if err != nil {
		t.Fatalf("GetInputFields() error = %v", err)
	}

	// HelloRequest has: name(string), age(int32), active(bool), score(double), address(message)
	if len(fields) < 5 {
		t.Errorf("expected at least 5 fields, got %d: %v", len(fields), fields)
	}

	fieldNames := map[string]bool{}
	for _, f := range fields {
		fieldNames[f.Name] = true
	}
	for _, expected := range []string{"name", "age", "active", "score", "address"} {
		if !fieldNames[expected] {
			t.Errorf("missing expected field %q", expected)
		}
	}
}
