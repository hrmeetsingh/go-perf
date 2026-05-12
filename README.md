# go-perf

A standalone performance benchmarking tool for Go gRPC services, built on top of [ghz](https://ghz.sh/).

## Features

- **Proto-aware**: Automatically discovers and parses `.proto` files — supports a single file or recursive folder scan. No `protoc` dependency (uses [protoreflect](https://github.com/jhump/protoreflect)).
- **Dynamic payloads**: Generate sample payloads from proto definitions. Mark fields as dynamic with random generators (UUID, int range, random string, timestamp) or user-defined value pools.
- **Multi-call benchmarking**: Benchmark multiple RPC methods from the same proto file in one run, each with its own `dynamic_fields` configuration. Run them sequentially or in parallel.
- **Multi-variant parallel benchmarks**: Run multiple payload variants simultaneously using goroutines, each backed by a ghz runner instance. Observe how your service handles mixed traffic patterns.
- **Payload-to-latency drill-down**: Each request is tagged with a payload hash. Reports group latency metrics by payload variant so you can identify which payload shapes cause spikes.
- **Multiple report formats**: CLI table, HTML (dark-themed responsive design), JSON, and JUnit XML. HTML reports can be printed to PDF from any browser.
- **Benchmark storage & comparison**: Save results as timestamped JSON files. Compare any two runs to see regressions or improvements in latency, RPS, and error rates.
- **JWT authentication**: Static tokens via flag/config, or automatic OAuth2/OIDC `client_credentials` token fetching.
- **Unary & bidirectional streaming**: Supports both RPC patterns with configurable send rate and concurrent stream count for streaming.
- **CI/CD pipeline integration**: JSON output to stdout + JUnit XML for CI dashboards. Non-zero exit on configuration errors. Machine-readable output for automated threshold checks.

## Installation

```bash
go install github.com/hrmeetsingh/go-perf@latest
```

Or build from source:

```bash
git clone https://github.com/hrmeetsingh/go-perf.git
cd go-perf
go build -o go-perf .
```

## Quick Start

### Run a benchmark

```bash
# Single call — all flags
go-perf run \
  --target localhost:50051 \
  --proto ./protos/service.proto \
  --call mypackage.MyService/DoWork \
  --total 1000 \
  --concurrency 50

# With a config file (single or multiple calls)
go-perf run -c config.yaml

# Override the call list to a single method at runtime
go-perf run -c config.yaml --call mypackage.MyService/DoWork

# Run multiple calls in parallel
go-perf run -c config.yaml --parallel

# With JWT auth and saved results
go-perf run -c config.yaml --token "eyJ..." --save --format cli,json,junit
```

### Generate a sample payload

```bash
go-perf generate \
  --proto ./protos/service.proto \
  --service mypackage.MyService \
  --method DoWork
```

Output:

```json
{
  "name": "sample_name",
  "age": 0,
  "active": false,
  "score": 0,
  "address": {
    "street": "sample_street",
    "city": "sample_city",
    "zip": 0
  }
}
```

### Compare benchmarks

```bash
# List stored benchmarks
go-perf list

# Compare two runs
go-perf compare .go-perf/benchmarks/run_20260510T143000_run-1.json \
                .go-perf/benchmarks/run_20260510T150000_run-2.json
```

### Regenerate reports

```bash
go-perf report .go-perf/benchmarks/run_20260510T143000_run-1.json --format cli,html
```

## Configuration

Copy `config.example.yaml` to `go-perf.yaml` and customize:

```yaml
target: "localhost:50051"
proto: "path/to/service.proto"

# Benchmark parameters — shared across all calls
concurrency: 50
total: 1000
duration: "30s"
connections: 5
timeout: "10s"

# Run all calls in parallel (default: false = sequential)
parallel: false

metadata:
  x-request-id: "bench-run"

auth:
  type: "static"              # "static" or "oauth"
  token: "your-jwt-token"
  # token_url: "https://auth.example.com/oauth/token"
  # client_id: "my-client-id"
  # client_secret: "my-client-secret"

# List of RPC methods to benchmark — each with its own dynamic_fields
calls:
  - call: "mypackage.MyService/DoWork"
    dynamic_fields:
      - field: "user_id"
        type: "uuid"
      - field: "amount"
        type: "int_range"
        min: 1
        max: 10000
      - field: "status"
        type: "pool"
        values: ["active", "inactive", "pending"]

  - call: "mypackage.MyService/GetStatus"
    dynamic_fields:
      - field: "name"
        type: "string"
        length: 12
      - field: "created_at"
        type: "timestamp"

output:
  formats: ["cli", "html", "json", "junit"]
  dir: "./reports"

stream:
  send_rate: 100
  stream_count: 5
```

CLI flags override YAML values. Run `go-perf run --help` for all available flags.

### Multiple Calls

The `calls:` list lets you benchmark several RPC methods from the same proto file in a single `go-perf run`. Each entry specifies:

- `call` — fully qualified method name (`pkg.Service/Method`)
- `dynamic_fields` — payload randomisation rules **scoped to this call only**

```yaml
calls:
  - call: "billing.PaymentService/Charge"
    dynamic_fields:
      - field: "amount"
        type: "int_range"
        min: 100
        max: 50000
      - field: "currency"
        type: "pool"
        values: ["USD", "EUR", "GBP"]

  - call: "billing.PaymentService/Refund"
    dynamic_fields:
      - field: "reason"
        type: "pool"
        values: ["duplicate", "fraudulent", "requested_by_customer"]
```

By default calls run sequentially. Add `parallel: true` (or `--parallel`) to run them concurrently:

```bash
go-perf run -c config.yaml --parallel
```

The `--call` CLI flag accepts a single method and collapses the `calls:` list to that one entry, useful for one-off investigations without editing the config file.

### Dynamic Field Types

| Type        | Description                          | Parameters        |
|-------------|--------------------------------------|-------------------|
| `uuid`      | Random UUID v4 per request           | —                 |
| `int_range` | Random integer in [min, max]         | `min`, `max`      |
| `pool`      | Pick from a list of values           | `values`          |
| `string`    | Random alphanumeric string           | `length` (default 10) |
| `timestamp` | Current Unix nanosecond timestamp    | —                 |

## Running Tests

```bash
go test ./... -count=1
```

Test coverage spans all internal packages:

| Package   | Tests | Coverage |
|-----------|-------|----------|
| config    | 17    | Config loading, multi-call validation, merging, defaults, parallel/call overrides |
| proto     | 11    | File discovery, parsing, service/method resolution, field extraction |
| payload   | 14    | Generation, hashing, all 5 provider types, dynamic field application |
| runner    | 9     | Config validation, latency stats, grouping, merging |
| auth      | 11    | Static/OAuth providers, metadata, error handling |
| report    | 12    | CLI/JSON/HTML/JUnit reporters, multi-reporter, WriteMultiCall |
| storage   | 15    | Save/load/list, BenchmarkRun save/load, comparison, summary |

## Architecture

```
go-perf/
├── cmd/                        # Cobra CLI commands
│   ├── root.go                 # Root command, global --config flag
│   ├── run.go                  # Multi-call benchmark execution
│   ├── generate.go             # Proto → sample payload
│   ├── compare.go              # Benchmark comparison
│   ├── report.go               # Regenerate reports from stored data
│   └── list.go                 # List stored benchmarks
├── internal/
│   ├── config/                 # YAML parsing, flag merging, validation, defaults
│   ├── proto/                  # Proto discovery, parsing (protoreflect), type mapping
│   ├── payload/                # Payload generation, hashing, dynamic providers, factory
│   ├── runner/                 # Engine interface, GhzEngine, Orchestrator, result types
│   ├── auth/                   # Static token + OAuth2 client_credentials providers
│   ├── report/                 # CLI/JSON/HTML/JUnit reporters, factory, multi-reporter
│   └── storage/                # JSON file store, BenchmarkRun, benchmark comparison
├── templates/                  # HTML report template
├── main.go                     # Entry point
├── config.example.yaml         # Documented example configuration
├── CHANGELOG.md                # TDD step-by-step changelog
└── go.mod
```

### Key Design Decisions

1. **ghz as Go library**: Imported `github.com/bojand/ghz/runner` directly — no shell-out, full control over options, native Go types for results.

2. **Engine interface**: The `runner.Engine` interface abstracts the benchmark backend. `GhzEngine` is the production implementation; mock engines can be injected for testing orchestrator logic without a live gRPC server.

3. **Goroutine orchestration**: The `Orchestrator` spawns one goroutine per payload variant, each running its own ghz instance. Results are collected under mutex and merged. This tests how the service handles mixed concurrent traffic patterns.

4. **Payload hashing for drill-down**: Each request's payload variant is identified by a hash. The result collector groups latency records by hash, enabling per-variant analysis in reports without storing full payloads (lightweight approach).

5. **No protoc dependency**: Using `jhump/protoreflect` for pure-Go proto parsing. Users don't need protoc installed.

6. **Factory patterns**: `payload.NewProviderFromConfig` and `report.NewReporterFromFormat` centralize construction logic, making it easy to add new provider types or report formats.

7. **Multi-call with per-call dynamic fields**: The `calls:` list replaces the previous single `call:` field. Each `CallEntry` carries its own `dynamic_fields`, so different methods can have independent payload variation strategies in the same run. Sequential or parallel execution is controlled by `parallel:` / `--parallel`.

8. **BenchmarkRun storage**: A single `go-perf run` that exercises multiple calls saves one `BenchmarkRun` JSON file containing all call results, rather than one file per call. This keeps the store directory tidy and makes multi-call comparison straightforward.

## CI/CD Integration

```yaml
# GitHub Actions example
- name: Run gRPC benchmark
  run: |
    go-perf run \
      -c bench-config.yaml \
      --format json,junit \
      --output-dir ./bench-reports \
      --save

- name: Upload JUnit results
  uses: dorny/test-reporter@v1
  with:
    name: Benchmark Results
    path: ./bench-reports/report.xml
    reporter: java-junit

- name: Compare with baseline
  run: |
    go-perf compare \
      .go-perf/benchmarks/baseline.json \
      .go-perf/benchmarks/latest.json
```

## Example: Sample gRPC Service

The `examples/` directory contains a self-contained gRPC service designed to demonstrate every feature of go-perf. It simulates realistic, varied performance so benchmark reports are visually interesting.

### What the example service does

**Proto file:** `examples/bench.proto`

| RPC | Type | Base latency | Performance pattern |
|-----|------|-------------|---------------------|
| `FastEcho` | Unary | ~2ms | 1-in-10 requests spike to 20ms |
| `ProcessOrder` | Unary | 5ms + 1ms/100B payload | Payload size + user tier driven; 1-in-10 spikes |
| `StreamEvents` | Server-stream | ~10ms/message | 1-in-5 messages delayed to 50ms |
| `BidirectionalChat` | Bidi-stream | ~3ms/message | User-tier multiplied; 1-in-8 spike to 30ms |

**User tiers** — pass `user_tier` field as `premium`, `standard`, or `free`:

| Tier | Latency multiplier |
|------|--------------------|
| premium | 1× (fastest) |
| standard | 2× |
| free | 5× (slowest) |

### Running the full example

**Step 1 — Build and start the server** (terminal 1):

```bash
make server
# or
go run examples/server/main.go
```

Output:
```
BenchService listening on :50051
  FastEcho       — unary,  ~2ms base, 1-in-10 spikes
  ProcessOrder   — unary,  payload-size + tier driven
  StreamEvents   — server-stream, ~10ms/message
  BidirectionalChat — bidi, ~3ms/message, tier driven
```

**Step 2 — Run benchmarks** (terminal 2):

```bash
# Quick sanity check — 200 requests, CLI output
make bench-quick

# Full benchmark: ProcessOrder + FastEcho sequentially, all output formats
make bench-full
# → reports at ./reports/full/report.html

# Dynamic payload benchmark: FastEcho + ProcessOrder in parallel,
# per-call per-tier drill-down
make bench-dynamic
# → drill-down report shows per-tier latency at ./reports/dynamic/report.html

# Two sequential runs then a comparison
make bench-compare

# Bidirectional streaming — 30 seconds
make bench-stream
```

**Generate a sample payload from proto:**

```bash
make generate-payload
```

Output:
```json
{
  "order_id": "sample_order_id",
  "amount": 0,
  "user_tier": "sample_user_tier",
  "payload": ""
}
```

### Makefile reference

```
make help              Show all available targets
make build             Compile go-perf binary
make test              Run all tests (97 total)
make server            Start sample gRPC server on :50051
make bench-quick       200-request unary benchmark, CLI output
make bench-full        ProcessOrder + FastEcho sequentially, all formats
make bench-dynamic     FastEcho + ProcessOrder in parallel, per-call drill-down
make bench-compare     Two runs followed by comparison diff
make bench-stream      30s bidirectional streaming benchmark
make generate-payload  Extract sample payload from bench.proto
make proto             Regenerate Go code from bench.proto (needs protoc)
make clean             Remove binary, reports, benchmarks
```

### Example benchmark report output

After `make bench-dynamic`, the CLI shows a separate section per call, each with its own payload-hash breakdown:

```
============================================================
  Multi-Call gRPC Benchmark Report — localhost:50051
============================================================

============================================================
  gRPC Benchmark Report
============================================================

  Target:       localhost:50051
  Call:         bench.BenchService/FastEcho
  Total:        600    Errors: 0    RPS: 142.30

  Latency:
    Min:    1.8ms    Avg: 4.1ms    P50: 2.3ms
    P95:   18.4ms    P99: 21.7ms   Max: 22.1ms

  Payload Variant Breakdown:
  Hash             Count    Avg          P99          Max
  ------------------------------------------------------------
  a1b2c3d4e5f6     189      2.1ms        4.8ms        5.2ms   ← premium
  f6e5d4c3b2a1     201      4.3ms        9.1ms        9.8ms   ← standard
  c3d4e5f6a1b2     210     10.8ms       21.7ms       22.1ms   ← free

============================================================
  gRPC Benchmark Report
============================================================

  Target:       localhost:50051
  Call:         bench.BenchService/ProcessOrder
  Total:        600    Errors: 0    RPS: 98.40

  Latency:
    Min:    4.2ms    Avg: 9.7ms    P50: 7.1ms
    P95:   35.2ms    P99: 48.3ms   Max: 52.1ms

  Payload Variant Breakdown:
  Hash             Count    Avg          P99          Max
  ------------------------------------------------------------
  b2c3d4e5f6a1     194      5.6ms       12.1ms       13.4ms   ← premium
  e5f6a1b2c3d4     198      9.8ms       24.7ms       27.2ms   ← standard
  d4e5f6a1b2c3     208     28.4ms       48.3ms       52.1ms   ← free
```

The payload hash groups map back to the dynamic `user_tier` pool values, letting you instantly see which tier caused the P99 spike — per call.

## License

MIT
