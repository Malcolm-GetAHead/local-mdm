# F-05: Advanced Monitoring & Observability

**Priority**: Low  
**Effort**: 2-3 days  
**Score Impact**: +0.08 points  
**Status**: Beyond v1.0 scope

---

## Gap Analysis

### Current State
- Prometheus metrics (S5-06)
- Structured JSON logging (S5-06)
- Health checks with dependency polling (S5-06)
- Alerting documentation (S5-06)

### Missing
- Distributed tracing (OpenTelemetry, Jaeger)
- Application Performance Monitoring (APM)
- Real User Monitoring (RUM) for web dashboard
- Anomaly detection (ML-based alerting)
- SLO/SLI definitions
- Error budget tracking
- Detailed runbooks per alert

### Impact
Without advanced monitoring:
- Difficult to debug cross-service issues
- No visibility into request flows
- Performance bottlenecks hard to identify
- Reactive alerting (not predictive)
- No SLO tracking for reliability goals

---

## Proposed Solution

### 1. Distributed Tracing

**OpenTelemetry Integration**:
```go
// internal/tracing/tracing.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing(serviceName string) error {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
    ))
    if err != nil {
        return err
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return nil
}
```

**Instrumentation**:
```go
// internal/api/handlers.go
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
    ctx, span := otel.Tracer("api").Start(r.Context(), "GetDevice")
    defer span.End()
    
    deviceID := chi.URLParam(r, "id")
    span.SetAttributes(attribute.String("device.id", deviceID))
    
    // Call service with traced context
    device, err := h.deviceService.GetByID(ctx, deviceID)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return
    }
    
    span.SetAttributes(attribute.String("device.platform", device.Platform))
    json.NewEncoder(w).Encode(device)
}
```

**Trace Visualization**:
```
Request: GET /api/v1/devices/123
├── api.GetDevice (15ms)
│   ├── service.GetByID (12ms)
│   │   ├── repository.GetByID (10ms)
│   │   │   └── database.Query (8ms)
│   │   └── cache.Get (2ms)
│   └── json.Encode (3ms)
```

### 2. Application Performance Monitoring (APM)

**Options**:
- **Elastic APM** (open source)
- **Datadog APM** (commercial)
- **New Relic** (commercial)
- **Grafana Tempo** (open source)

**Elastic APM Integration**:
```go
// internal/apm/apm.go
import "go.elastic.co/apm/module/apmhttp"

func WrapHandler(h http.Handler) http.Handler {
    return apmhttp.Wrap(h)
}
```

**Metrics Tracked**:
- Request rate (requests/second)
- Response time (p50, p95, p99)
- Error rate (errors/second)
- Throughput (bytes/second)
- Database query time
- External API call time
- Memory usage
- CPU usage
- Goroutine count

### 3. Real User Monitoring (RUM)

**Web Dashboard Monitoring**:
```javascript
// web/src/monitoring.js
import { init as initApm } from '@elastic/apm-rum'

const apm = initApm({
  serviceName: 'localmdm-dashboard',
  serverUrl: 'https://apm.example.com',
  environment: 'production'
})

// Track page loads
apm.setInitialPageLoadName('Dashboard')

// Track user interactions
document.getElementById('lock-device').addEventListener('click', () => {
  const transaction = apm.startTransaction('Lock Device', 'user-interaction')
  // ... perform action
  transaction.end()
})
```

**Metrics Tracked**:
- Page load time
- Time to interactive
- First contentful paint
- Largest contentful paint
- Cumulative layout shift
- User interactions (clicks, form submissions)
- JavaScript errors
- API call latency (from browser)

### 4. Anomaly Detection

**ML-Based Alerting**:
```python
# scripts/anomaly_detection.py
from prometheus_api_client import PrometheusConnect
from sklearn.ensemble import IsolationForest
import numpy as np

prom = PrometheusConnect(url="http://prometheus:9090")

# Fetch enrollment rate for last 7 days
metric_data = prom.get_metric_range_data(
    metric_name='mdm_enrollments_total',
    start_time='7d',
    end_time='now'
)

# Train anomaly detection model
values = np.array([point[1] for point in metric_data])
model = IsolationForest(contamination=0.1)
model.fit(values.reshape(-1, 1))

# Detect anomalies in current data
current_value = prom.get_current_metric_value('mdm_enrollments_total')
is_anomaly = model.predict([[current_value]])[0] == -1

if is_anomaly:
    send_alert("Anomalous enrollment rate detected")
```

**Anomalies to Detect**:
- Sudden spike in enrollment failures
- Unusual API error rate
- Abnormal database query latency
- Unexpected traffic patterns
- Memory leak detection
- CPU usage spikes

### 5. SLO/SLI Definitions

**Service Level Indicators (SLIs)**:

| SLI | Measurement | Target |
|-----|-------------|--------|
| Availability | Successful requests / Total requests | 99.9% |
| Latency | p95 response time | < 200ms |
| Error Rate | Failed requests / Total requests | < 0.1% |
| Enrollment Success | Successful enrollments / Total attempts | > 95% |
| Command Success | Successful commands / Total commands | > 98% |

**Service Level Objectives (SLOs)**:

```yaml
# slo.yaml
slos:
  - name: API Availability
    sli: http_requests_total{status!~"5.."}
    target: 99.9%
    window: 30d
    
  - name: API Latency
    sli: histogram_quantile(0.95, http_request_duration_seconds)
    target: 200ms
    window: 30d
    
  - name: Enrollment Success Rate
    sli: mdm_enrollments_total{status="success"}
    target: 95%
    window: 7d
```

**Error Budget**:
```
SLO: 99.9% availability over 30 days
Total time: 30 days = 43,200 minutes
Allowed downtime: 0.1% = 43.2 minutes
Error budget: 43.2 minutes/month

Current month:
- Downtime so far: 15 minutes
- Remaining budget: 28.2 minutes
- Budget consumed: 34.7%
```

**Error Budget Policy**:
- **Budget > 50%**: Normal operations, deploy freely
- **Budget 25-50%**: Caution, reduce deployment frequency
- **Budget < 25%**: Freeze deployments, focus on reliability
- **Budget exhausted**: Emergency only, incident review required

### 6. Enhanced Alerting

**Alert Runbooks**:

```yaml
# alerts/high_error_rate.yaml
alert: HighErrorRate
expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
for: 5m
severity: critical
runbook: |
  # High Error Rate Runbook
  
  ## Symptoms
  - API returning 5xx errors at high rate (> 5%)
  
  ## Investigation
  1. Check application logs: kubectl logs -l app=localmdm --tail=100
  2. Check database connectivity: curl http://localmdm/health/ready
  3. Check recent deployments: kubectl rollout history deployment/localmdm
  4. Check resource usage: kubectl top pods -l app=localmdm
  
  ## Resolution
  - If database issue: Check database status, failover if needed
  - If deployment issue: Rollback to previous version
  - If resource issue: Scale up replicas or increase limits
  
  ## Escalation
  - If unresolved in 15 minutes, page on-call manager
  - If data loss suspected, page database team
```

**Alert Grouping**:
```yaml
# alertmanager.yaml
route:
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'pagerduty'
  
  routes:
  - match:
      severity: critical
    receiver: 'pagerduty'
    continue: true
    
  - match:
      severity: warning
    receiver: 'slack'
```

---

## Implementation Tasks

### Task 1: Distributed Tracing (1 day)
- Install Jaeger or Tempo
- Integrate OpenTelemetry SDK
- Instrument API handlers
- Instrument service layer
- Instrument database calls
- Create trace dashboards

### Task 2: APM Integration (0.5 days)
- Choose APM solution (Elastic, Datadog, New Relic)
- Install APM agent
- Configure APM middleware
- Create APM dashboards
- Set up APM alerts

### Task 3: RUM for Dashboard (0.5 days)
- Integrate RUM library
- Track page loads and interactions
- Track JavaScript errors
- Create RUM dashboards
- Set up RUM alerts

### Task 4: Anomaly Detection (1 day)
- Set up ML pipeline
- Train anomaly detection models
- Create anomaly alerts
- Tune detection thresholds
- Document anomaly response

### Task 5: SLO/SLI Tracking (0.5 days)
- Define SLIs and SLOs
- Create SLO dashboards
- Implement error budget tracking
- Document error budget policy
- Set up SLO alerts

---

## Acceptance Criteria

- [ ] Distributed tracing captures all API requests
- [ ] Traces show end-to-end request flow
- [ ] APM tracks application performance
- [ ] RUM tracks web dashboard performance
- [ ] Anomaly detection alerts on unusual patterns
- [ ] SLOs defined and tracked
- [ ] Error budget calculated and monitored
- [ ] Alert runbooks created for all critical alerts

---

## Dashboards to Create

**1. Service Overview**:
- Request rate (by endpoint)
- Error rate (by endpoint)
- Latency (p50, p95, p99)
- Active users
- SLO compliance

**2. Trace Analysis**:
- Slowest traces
- Failed traces
- Trace duration distribution
- Service dependency map

**3. Error Budget**:
- Current error budget
- Budget burn rate
- Historical budget usage
- SLO compliance over time

**4. User Experience**:
- Page load times
- User interactions
- JavaScript errors
- API latency (from browser)

---

## Cost Considerations

**Open Source** (self-hosted):
- Jaeger: Free
- Grafana Tempo: Free
- Elastic APM: Free
- Infrastructure: $200-500/month

**Commercial**:
- Datadog APM: $15-31/host/month
- New Relic: $99-349/user/month
- Elastic Cloud: $95-175/month

**Total**: $0-1,000/month (depending on choice)

---

## Future Enhancements

- Predictive alerting (ML-based)
- Automated root cause analysis
- Continuous profiling
- Cost monitoring and optimization
- User journey tracking
- A/B testing framework

---

## References

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [SLO/SLI Best Practices](https://sre.google/workbook/implementing-slos/)
- [S5-06: Observability](../sprint-5-ui-and-polish/S5-06-observability.md)
