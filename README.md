# UUID Compatibility & Throughput Validation Tool

Validates whether `github.com/google/uuid` can be replaced by the Go standard-library `uuid` package (Go 1.27+) in `clickhouse-go` (see [clickhouse-go discussion #1895](https://github.com/ClickHouse/clickhouse-go/discussions/1895)) without breaking compatibility or introducing a significant throughput regression.

The tool tests the **real driver path**: native ClickHouse `UUID` columns, batch insertion and per-row scanning through `clickhouse-go` — no `String`/`FixedString`/`[]byte` escapes, no application-level string conversion.

## Requirements

- Go 1.27 toolchain (the stdlib `uuid` package landed in Go 1.27; `go.mod` pins `toolchain go1.27rc3` and downloads it automatically via `GOTOOLCHAIN=auto`)
- A running ClickHouse (native port 9000). Example with Docker:

```bash
docker run -d --name uuid-ch -p 9000:9000 -p 8123:8123 \
  --ulimit nofile=262144:262144 \
  -e CLICKHOUSE_PASSWORD=test \
  clickhouse/clickhouse-server:latest
```

## Build

```bash
go build ./cmd/uuid-compat-bench
go test ./...
```

## Test Matrix

| Scenario        | Producer    | Consumer    | Purpose                     |
| --------------- | ----------- | ----------- | --------------------------- |
| Baseline        | google/uuid | google/uuid | Existing behavior baseline  |
| Candidate       | stdlib uuid | stdlib uuid | New implementation behavior |
| Cross A         | google/uuid | stdlib uuid | Compatibility validation    |
| Cross B         | stdlib uuid | google/uuid | Compatibility validation    |

Each scenario tests two decode paths:

- **native** — the driver scans directly into each implementation's native UUID type (the path a patched driver must support; fails with `clickhouse-go` < the stdlib-UUID change)
- **bridge** — control test via the driver's string decode + consumer `Parse` (proves data integrity independently of driver type support)

## Usage

```bash
# Full validation (default): compatibility + cross-check + throughput
uuid-compat-bench run --rows 10000000 --concurrency 1,2,4,8,16,32,64,128

# Compatibility / correctness only
uuid-compat-bench compatibility --rows 1000000

# Throughput only
uuid-compat-bench benchmark --rows 10000000 --duration 30s --warmup 10s
```

Options:

```text
--host localhost     ClickHouse host                  (env CLICKHOUSE_HOST)
--port 9000          native port                      (env CLICKHOUSE_PORT)
--database default                                     (env CLICKHOUSE_DATABASE)
--username default                                     (env CLICKHOUSE_USERNAME)
--password ""                                          (env CLICKHOUSE_PASSWORD)
--secure            use TLS
--rows 1000000      rows per scenario
--table NAME        table name prefix (scenario tables: NAME_{gg,ss,gs,sg})
--uuid-version v4   v4 or v7
--concurrency       1,2,4,8,16,32,64,128
--warmup 10s
--duration 30s      measurement per iteration
--iterations 3      median of iterations is the reported result
--warn-regression 5  regression % → WARNING
--fail-regression 10 regression % → FAIL
--output text|json  machine-readable verdict for CI
```

Environment variables are overridden by explicit CLI flags.

## Verdicts

- Compatibility: all four scenarios must pass (matched == inserted, zero errors) in the native path
- Performance: regression % of `stdlib -> stdlib` vs `google -> google` peak insert throughput; `> warn` → `WARNING`, `> fail` → `FAIL`
- Exit codes: `0` = PASS (or WARNING), `1` = FAIL — CI friendly:

```bash
uuid-compat-bench run --rows 100000 --duration 10s --output json; echo $?
```

## Example Output (text)

```text
Concurrency  google -> google   stdlib -> stdlib   ...
1             120K/s             121K/s
...
Regression
----------
stdlib vs google baseline (2.31M/s vs 2.29M/s):
  0.87%
  thresholds: warn > 5%, fail > 10%

Verdict
-------
PASS
```

## Current Status (clickhouse-go v2.48.0)

Running the tool against the unmodified upstream driver reports:

- `google -> google` / `stdlib -> google` — **PASS** (stdlib inserts via the driver's Stringer bridge)
- `stdlib -> stdlib` / `google -> stdlib` — **FAIL** on the native decode path: `clickhouse-go` does not yet scan UUID columns into the stdlib `uuid.UUID` type (bridge control still PASSes, so the data itself is intact)

## Validating the Driver Change (fork + replace)

`clickhouse-go` discussion [#1895](https://github.com/ClickHouse/clickhouse-go/discussions/1895) proposes replacing the `google/uuid` dependency with the stdlib `uuid` package. The change has been implemented on the `stdlib-uuid` branch / `v2.49.0-stdlib.1` tag of `github.com/Laotree/clickhouse-go` and can be validated before it lands upstream via a `go.mod` replace:

```bash
go mod edit -replace=github.com/ClickHouse/clickhouse-go/v2=github.com/Laotree/clickhouse-go/v2@v2.49.0-stdlib.1
go mod tidy
uuid-compat-bench run --rows 10000000
```

Result with the patched driver:

- All four scenarios **PASS** on the native path (stdlib `uuid.UUID` now scans directly into the driver's `ScanRow`; `google/uuid` values still work via `sql.Scanner`/`driver.Valuer`).
- Regression of `stdlib -> stdlib` vs `google -> google` is **negative** (stdlib is faster, since inserts no longer round-trip through the driver's string bridge).
- Verdict **PASS** with exit code 0, ready for CI.

Note: `github.com/google/uuid` remains an indirect dependency of the fork because `ch-go`'s `proto.ColUUID` is still typed `google/uuid.UUID`; the fork converts at that boundary.

## Project Layout

```text
cmd/uuid-compat-bench/   CLI entry
internal/uuid/           provider abstraction (native types preserved)
internal/clickhouse/     connection, schema, batch/query helpers
internal/benchmark/      compatibility matrix + throughput engine + verdicts
internal/report/         text/JSON output, environment info
tests/                   integration tests (require ClickHouse + CLICKHOUSE_PASSWORD)
sql/schema.sql           reference DDL
```