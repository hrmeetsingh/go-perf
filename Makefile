BINARY    := go-perf
SERVER    := examples/server/main.go
PROTO     := examples/bench.proto
PROTO_OUT := examples/gen/bench
PORT      := 50051
STORE_DIR := .go-perf/benchmarks

.PHONY: all build test clean server \
        bench-quick bench-full bench-dynamic bench-compare \
        generate-payload proto help

all: build

## help: Show this help message
help:
	@echo ""
	@echo "  go-perf — gRPC Performance Benchmarking Tool"
	@echo ""
	@echo "  Setup"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "    %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

## build: Compile the go-perf binary
build:
	go build -o $(BINARY) .

## test: Run all unit tests
test:
	go test ./... -count=1

## clean: Remove binary, reports, and stored benchmarks
clean:
	rm -f $(BINARY)
	rm -rf reports/ $(STORE_DIR)/

## proto: Regenerate Go code from bench.proto (requires protoc)
proto:
	@which protoc > /dev/null || (echo "protoc not found. Install: https://grpc.io/docs/protoc-installation/" && exit 1)
	@which protoc-gen-go > /dev/null || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@which protoc-gen-go-grpc > /dev/null || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	mkdir -p $(PROTO_OUT)
	PATH="$$(go env GOPATH)/bin:$$PATH" protoc \
		--go_out=$(PROTO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) \
		--go-grpc_opt=paths=source_relative \
		--proto_path=examples \
		$(PROTO)
	@echo "Generated: $(PROTO_OUT)/bench.pb.go $(PROTO_OUT)/bench_grpc.pb.go"

## server: Start the sample gRPC server (port 50051)
server:
	@echo "Starting BenchService on :$(PORT)..."
	@echo "  FastEcho       — unary,  ~2ms base, 1-in-10 spikes"
	@echo "  ProcessOrder   — unary,  payload-size + tier driven"
	@echo "  StreamEvents   — server-stream, ~10ms/message"
	@echo "  BidirectionalChat — bidi, ~3ms/message, tier driven"
	@echo ""
	go run $(SERVER)

## generate-payload: Generate a sample JSON payload from bench.proto
generate-payload: build
	./$(BINARY) generate \
		--proto $(PROTO) \
		--service bench.BenchService \
		--method ProcessOrder

## bench-quick: Quick 200-request unary benchmark (CLI output)
bench-quick: build
	@echo "Running quick benchmark (FastEcho, 200 req)..."
	./$(BINARY) run -c examples/configs/quick.yaml --insecure

## bench-full: Full 2000-request benchmark with all output formats
bench-full: build
	@echo "Running full benchmark (ProcessOrder, 2000 req, all formats)..."
	./$(BINARY) run -c examples/configs/full.yaml --insecure --save --store-dir $(STORE_DIR)
	@echo ""
	@echo "Reports written to ./reports/full/"
	@echo "Open ./reports/full/report.html in a browser to view the full report."

## bench-dynamic: Dynamic payload benchmark showing per-tier drill-down
bench-dynamic: build
	@echo "Running dynamic payload benchmark (FastEcho, 3 user tiers)..."
	./$(BINARY) run -c examples/configs/dynamic.yaml --insecure --save --store-dir $(STORE_DIR)
	@echo ""
	@echo "Payload hash drill-down available in ./reports/dynamic/report.html"

## bench-compare: Run two sequential benchmarks then diff them
bench-compare: build
	@echo "Run 1 of 2 (baseline)..."
	./$(BINARY) run -c examples/configs/quick.yaml --insecure --save --store-dir $(STORE_DIR)
	@sleep 2
	@echo "Run 2 of 2 (current)..."
	./$(BINARY) run -c examples/configs/quick.yaml --insecure --save --store-dir $(STORE_DIR)
	@echo ""
	@echo "Comparing last two stored benchmarks..."
	@RUNS=$$(ls -t $(STORE_DIR)/*.json 2>/dev/null | head -2); \
	if [ $$(echo "$$RUNS" | wc -l | tr -d ' ') -lt 2 ]; then \
		echo "Need at least 2 stored runs. Run bench-compare again."; \
	else \
		CURRENT=$$(echo "$$RUNS" | head -1); \
		BASELINE=$$(echo "$$RUNS" | tail -1); \
		./$(BINARY) compare "$$BASELINE" "$$CURRENT"; \
	fi

## bench-stream: Bidirectional streaming benchmark (30s)
bench-stream: build
	@echo "Running bidi streaming benchmark (30s)..."
	./$(BINARY) run -c examples/configs/stream.yaml --insecure
