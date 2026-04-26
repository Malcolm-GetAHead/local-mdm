# Load Testing (k6)

Performance tests for Local MDM using [k6](https://k6.io/).

## Prerequisites

```bash
# Install k6 (macOS)
brew install k6

# Or via Docker
docker run --rm -i grafana/k6 run - <scenario.js
```

## Scenarios

| Scenario | Description | Target |
|----------|-------------|--------|
| `steady_state.js` | Simulated device check-ins over time | 1000 VUs, 5 min |
| `admin_dashboard.js` | Admin API browsing (list devices, policies, compliance) | 10 VUs, 2 min |
| `enrollment_burst.js` | Burst of device enrollments | 100 VUs, 2 min |

## Running

```bash
# From project root — runs all scenarios and records results
make load-test

# Individual scenario with history recording
./tests/load/run_and_record.sh tests/load/steady_state.js "Sprint 5b"

# Individual scenario without recording
k6 run tests/load/steady_state.js
```

## Results History

Results are appended to `results_history.csv` — a living document that tracks performance across sprints. Each row records:

| Column | Description |
|--------|-------------|
| `timestamp` | UTC time of the run |
| `git_ref` | Short commit hash |
| `sprint` | Sprint label or branch name |
| `scenario` | Which scenario was run |
| `p50_ms` | Median response time |
| `p95_ms` | 95th percentile response time |
| `p99_ms` | 99th percentile response time |
| `avg_ms` | Average response time |
| `total_requests` | Total completed requests |
| `error_rate_pct` | Percentage of failed requests |
| `rps` | Requests per second (throughput) |
| `json_file` | Path to raw k6 JSON output |

Raw JSON files are in `results/` (gitignored). The CSV is committed so trends are visible across sprints.

## Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| p95 response time | < 200ms | API endpoints under load |
| p99 response time | < 500ms | Acceptable tail latency |
| Error rate | < 1% | Non-5xx responses |
| Throughput | > 100 req/s | Sustained steady state |

## Comparing Across Sprints

```bash
# View the history
column -t -s, tests/load/results_history.csv

# Filter to a specific scenario
grep steady_state tests/load/results_history.csv | column -t -s,

# Check for regressions — p95 should stay under 200ms
awk -F, 'NR>1 && $6>200 {print "REGRESSION:", $0}' tests/load/results_history.csv
```
