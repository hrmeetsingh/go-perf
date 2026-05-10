package examples

import (
	"os"
	"strings"
	"testing"
)

func TestMakefile_RequiredTargets(t *testing.T) {
	data, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("reading Makefile: %v", err)
	}
	content := string(data)

	required := []string{
		"build",
		"test",
		"server",
		"bench-quick",
		"bench-full",
		"bench-dynamic",
		"bench-compare",
		"generate-payload",
		"clean",
	}

	for _, target := range required {
		// Makefile targets appear as "target:" at start of line
		if !strings.Contains(content, target+":") {
			t.Errorf("Makefile missing required target %q", target)
		}
	}
}

func TestMakefile_HelpTarget(t *testing.T) {
	data, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("reading Makefile: %v", err)
	}
	if !strings.Contains(string(data), "help") {
		t.Error("Makefile should have a help target")
	}
}

func TestBenchProtoExists(t *testing.T) {
	if _, err := os.Stat("bench.proto"); os.IsNotExist(err) {
		t.Error("bench.proto does not exist in examples/")
	}
}

func TestGeneratedGoFilesExist(t *testing.T) {
	files := []string{
		"gen/bench/bench.pb.go",
		"gen/bench/bench_grpc.pb.go",
	}
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("generated file %q does not exist", f)
		}
	}
}

func TestServerMainExists(t *testing.T) {
	if _, err := os.Stat("server/main.go"); os.IsNotExist(err) {
		t.Error("server/main.go does not exist")
	}
}
