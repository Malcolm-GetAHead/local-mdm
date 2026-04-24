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
# From project root — targets the Docker stack
make load-test

# Individual scenario
k6 run tests/load/steady_state.js

# With custom base URL
k6 run -e BASE_URL=http://localhost:8080 tests/load/steady_state.js
```

## Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| p95 response time | < 200ms | API endpoints under load |
| p99 response time | < 500ms | Acceptable tail latency |
| Error rate | < 1% | Non-5xx responses |
| Throughput | > 100 req/s | Sustained steady state |

## Interpreting Results

k6 outputs a summary after each run. Key metrics:
- `http_req_duration`: Response time distribution (p50, p95, p99)
- `http_req_failed`: Error rate
- `iterations`: Total completed requests
- `vus`: Virtual users (concurrent connections)

Save results for comparison between sprints:
```bash
k6 run --out json=results/sprint-5b.json tests/load/steady_state.js
```
