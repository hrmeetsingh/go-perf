# go-perf

A standalone performance benchmarking tool for Go gRPC services, built on top of [ghz](https://ghz.sh/).

## Features

- **Proto-aware**: Automatically discovers and parses `.proto` files — supports a single file or recursive folder scan. No `protoc` dependency (uses [protoreflect](https://github.com/jhump/protoreflect)).
- **Dynamic payloads**: Generate sample payloads from proto definitions. Mark fields as dynamic with random generators (UUID, int range, random string, timestamp) or user-defined value pools.
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
# Minimal — all flags
go-perf run \
  --target localhost:50051 \
  --proto ./protos/service.proto \
  --call mypackage.MyService/DoWork \
  --total 1000 \
  --concurrency 50

# With a config file
go-perf run -c config.yaml

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
go-perf compare .go-perf/benchmarks/20260510T143000_run-1.json \
                .go-perf/benchmarks/20260510T150000_run-2.json
```

### Regenerate reports

```bash
go-perf report .go-perf/benchmarks/20260510T143000_run-1.json --format cli,html
```

## Configuration

Copy `config.example.yaml` to `go-perf.yaml` and customize:

```yaml
target: "localhost:50051"
proto: "path/to/service.proto"
call: "mypackage.MyService/DoWork"

concurrency: 50
total: 1000
duration: "30s"
connections: 5
timeout: "10s"

metadata:
  x-request-id: "bench-run"

auth:
  type: "static"              # "static" or "oauth"
  token: "your-jwt-token"
  # token_url: "https://auth.example.com/oauth/token"
  # client_id: "my-client-id"
  # client_secret: "my-client-secret"

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
go test ./internal/... -v
```

Test coverage spans all internal packages:

| Package   | Tests | Coverage |
|-----------|-------|----------|
| config    | 12    | Config loading, validation, merging, defaults |
| proto     | 11    | File discovery, parsing, service/method resolution, field extraction |
| payload   | 14    | Generation, hashing, all 5 provider types, dynamic field application |
| runner    | 9     | Config validation, latency stats, grouping, merging |
| auth      | 11    | Static/OAuth providers, metadata, error handling |
| report    | 9     | CLI/JSON/HTML/JUnit reporters, multi-reporter |
| storage   | 11    | Save/load/list, comparison, summary |

## Architecture

```
go-perf/
├── cmd/                        # Cobra CLI commands
│   ├── root.go                 # Root command, global --config flag
│   ├── run.go                  # Benchmark execution
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
│   └── storage/                # JSON file store, benchmark comparison
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

## License

MIT
