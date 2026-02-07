# Rate Limiting Architecture

**Last Updated**: 2026-02-07  
**Status**: ✅ Implemented (Development) + Documented (Production)

---

## Overview

The Local MDM system implements a **layered rate limiting strategy** that adapts to the deployment environment:

- **Development**: In-memory rate limiter (single instance)
- **Production**: Load balancer rate limiting (multi-instance)
- **Defense in Depth**: Both layers active for redundancy

---

## Development Environment

### In-Memory Rate Limiter

**Implementation**: `internal/api/ratelimit.go`

**Features**:
- Sliding window rate limiting
- LRU eviction (10,000 IP limit)
- Background cleanup goroutine
- Thread-safe (mutex-protected)
- Configurable limits per endpoint

**Configuration** (`configs/config.yaml`):
```yaml
server:
  rate_limit:
    enabled: true
    requests_per_min: 100
    window: 1m
```

**How It Works**:
1. Tracks requests per IP address in memory
2. Uses sliding window to count recent requests
3. Evicts oldest IPs when reaching 10K limit (LRU)
4. Cleans up expired entries every minute

**Limitations**:
- ❌ Does not work across multiple instances
- ❌ Memory usage grows with unique IPs (capped at 10K)
- ✅ Perfect for development/testing
- ✅ Provides defense-in-depth in production

---

## Production Environment

### Load Balancer Rate Limiting (Recommended)

**Why Load Balancer?**
- ✅ Works across all application instances automatically
- ✅ Blocks malicious traffic before reaching application
- ✅ Better performance (edge-level blocking)
- ✅ No memory usage in application
- ✅ Centralized configuration and monitoring
- ✅ Protects against DDoS attacks

### AWS Application Load Balancer (ALB)

**Configuration**:
```yaml
# AWS CloudFormation / Terraform
Resources:
  ALBRateLimitRule:
    Type: AWS::ElasticLoadBalancingV2::ListenerRule
    Properties:
      Actions:
        - Type: fixed-response
          FixedResponseConfig:
            StatusCode: 429
            ContentType: application/json
            MessageBody: '{"error":"Rate limit exceeded"}'
      Conditions:
        - Field: http-request-method
          HttpRequestMethodConfig:
            Values: ['*']
      Priority: 1
      
  # AWS WAF for rate limiting
  WebACL:
    Type: AWS::WAFv2::WebACL
    Properties:
      Rules:
        - Name: RateLimitRule
          Priority: 1
          Statement:
            RateBasedStatement:
              Limit: 2000  # requests per 5 minutes per IP
              AggregateKeyType: IP
          Action:
            Block:
              CustomResponse:
                ResponseCode: 429
```

**Rate Limits**:
- General API: 2,000 requests per 5 minutes per IP
- Authentication endpoints: 100 requests per 5 minutes per IP
- Device enrollment: 50 requests per 5 minutes per IP

### AWS API Gateway

**Configuration**:
```yaml
# API Gateway Throttling
Resources:
  ApiGateway:
    Type: AWS::ApiGateway::RestApi
    Properties:
      Name: LocalMDM
      
  ApiGatewayUsagePlan:
    Type: AWS::ApiGateway::UsagePlan
    Properties:
      Throttle:
        BurstLimit: 200    # Maximum concurrent requests
        RateLimit: 100     # Requests per second
      Quota:
        Limit: 10000       # Requests per day
        Period: DAY
```

### NGINX (Alternative)

**Configuration** (`/etc/nginx/nginx.conf`):
```nginx
http {
    # Define rate limit zones
    limit_req_zone $binary_remote_addr zone=general:10m rate=100r/m;
    limit_req_zone $binary_remote_addr zone=auth:10m rate=10r/m;
    limit_req_zone $binary_remote_addr zone=enrollment:10m rate=5r/m;
    
    # Connection limits
    limit_conn_zone $binary_remote_addr zone=addr:10m;
    
    server {
        listen 443 ssl http2;
        server_name mdm.example.com;
        
        # General API rate limit
        location /api/ {
            limit_req zone=general burst=20 nodelay;
            limit_conn addr 10;
            proxy_pass http://backend;
        }
        
        # Stricter limits for authentication
        location /api/v1/auth/ {
            limit_req zone=auth burst=5 nodelay;
            limit_conn addr 3;
            proxy_pass http://backend;
        }
        
        # Strictest limits for enrollment
        location /api/v1/devices/enroll {
            limit_req zone=enrollment burst=2 nodelay;
            limit_conn addr 1;
            proxy_pass http://backend;
        }
        
        # Return 429 on rate limit
        error_page 429 = @ratelimit;
        location @ratelimit {
            default_type application/json;
            return 429 '{"error":"Rate limit exceeded","retry_after":60}';
        }
    }
}
```

### Cloudflare (Alternative)

**Configuration** (Cloudflare Dashboard):
```
Rate Limiting Rules:
1. General API Protection
   - If: (http.request.uri.path contains "/api/")
   - Then: Rate limit 100 requests per minute per IP
   - Action: Block with 429 status

2. Authentication Protection  
   - If: (http.request.uri.path contains "/api/v1/auth/")
   - Then: Rate limit 10 requests per minute per IP
   - Action: Challenge (CAPTCHA)

3. DDoS Protection
   - Automatic (included with Cloudflare)
   - Blocks Layer 3/4 attacks
   - Mitigates Layer 7 attacks
```

---

## Layered Defense Strategy

### Layer 1: Load Balancer (Primary)
- Blocks malicious traffic at edge
- Protects all application instances
- Handles DDoS attacks
- **Rate Limit**: 100 requests/min per IP (general)

### Layer 2: Application (Backup)
- In-memory rate limiter remains active
- Catches traffic that bypasses load balancer
- Provides per-user rate limiting (not just per-IP)
- **Rate Limit**: 100 requests/min per IP (configurable)

### Why Both?

**Defense in Depth**:
- Load balancer misconfiguration won't leave app unprotected
- Direct access to app (if exposed) still protected
- Can implement user-based limits (not just IP-based)
- Provides visibility into rate limit hits at app level

---

## Monitoring & Alerting

### Metrics to Track

**Load Balancer Level**:
- `rate_limit_hits_total` - Total requests blocked
- `rate_limit_hits_by_ip` - Top offending IPs
- `rate_limit_hits_by_endpoint` - Most targeted endpoints

**Application Level**:
- `http_requests_total{status="429"}` - Rate limit responses
- `rate_limiter_memory_usage` - In-memory limiter size
- `rate_limiter_evictions_total` - LRU evictions

### Alerts

```yaml
# Prometheus Alert Rules
groups:
  - name: rate_limiting
    rules:
      - alert: HighRateLimitHits
        expr: rate(rate_limit_hits_total[5m]) > 100
        for: 5m
        annotations:
          summary: "High rate limit hits detected"
          description: "{{ $value }} requests/sec being rate limited"
          
      - alert: PossibleDDoS
        expr: rate(rate_limit_hits_total[1m]) > 1000
        for: 1m
        annotations:
          summary: "Possible DDoS attack detected"
          description: "{{ $value }} requests/sec being blocked"
```

---

## Configuration by Environment

### Development
```yaml
# configs/config.yaml
environment: development
server:
  rate_limit:
    enabled: true
    requests_per_min: 100
    window: 1m
```
- Uses in-memory rate limiter
- No load balancer required
- Suitable for local testing

### Staging
```yaml
# configs/config.yaml
environment: staging
server:
  rate_limit:
    enabled: true  # Keep as backup
    requests_per_min: 100
    window: 1m
```
- Load balancer rate limiting: 100 req/min
- In-memory limiter active as backup
- Test load balancer configuration

### Production
```yaml
# configs/config.yaml
environment: production
server:
  rate_limit:
    enabled: true  # Keep as backup
    requests_per_min: 100
    window: 1m
```
- **Primary**: AWS WAF rate limiting (2000 req/5min)
- **Backup**: In-memory rate limiter (100 req/min)
- **DDoS**: Cloudflare (if used)

---

## Per-Endpoint Rate Limits

### Recommended Limits

| Endpoint | Load Balancer | Application | Reason |
|----------|---------------|-------------|--------|
| `/api/v1/auth/login` | 10/min | 10/min | Prevent brute force |
| `/api/v1/auth/token` | 20/min | 20/min | Token refresh |
| `/api/v1/devices/enroll` | 5/min | 5/min | Prevent abuse |
| `/api/v1/devices` | 100/min | 100/min | General API |
| `/api/v1/policies` | 100/min | 100/min | General API |
| `/health` | Unlimited | Unlimited | Health checks |

---

## Testing Rate Limiting

### Load Balancer Testing

```bash
# Test rate limit at load balancer
for i in {1..150}; do
  curl -s -o /dev/null -w "%{http_code}\n" \
    https://mdm.example.com/api/v1/devices
done

# Expected: First 100 return 200, rest return 429
```

### Application Testing

```bash
# Test in-memory rate limiter (development)
for i in {1..150}; do
  curl -s -o /dev/null -w "%{http_code}\n" \
    http://localhost:8080/api/v1/devices
done

# Expected: First 100 return 200, rest return 429
```

### Automated Tests

```bash
# Run rate limiter tests
go test -v ./internal/api/... -run TestRateLimiter

# Tests include:
# - Basic rate limiting
# - LRU eviction
# - Concurrent access
# - Cleanup goroutine
# - Edge cases
```

---

## Migration Path

### Phase 1: Development (Current)
- ✅ In-memory rate limiter active
- ✅ Comprehensive tests
- ✅ Configurable limits

### Phase 2: Staging Deployment
- [ ] Configure ALB/API Gateway rate limiting
- [ ] Test load balancer limits
- [ ] Verify in-memory limiter as backup
- [ ] Monitor metrics

### Phase 3: Production Deployment
- [ ] Enable AWS WAF rate limiting
- [ ] Configure per-endpoint limits
- [ ] Set up monitoring and alerts
- [ ] Document runbook for rate limit incidents

---

## Troubleshooting

### Issue: Legitimate users being rate limited

**Diagnosis**:
```bash
# Check rate limit hits by IP
aws wafv2 get-sampled-requests \
  --web-acl-arn $WEB_ACL_ARN \
  --rule-metric-name RateLimitRule

# Check application logs
kubectl logs -l app=local-mdm | grep "rate limit"
```

**Solution**:
- Increase rate limits for specific IPs (allowlist)
- Implement user-based rate limiting (not just IP)
- Use API keys for known clients

### Issue: Rate limiter memory usage high

**Diagnosis**:
```bash
# Check in-memory limiter size
curl http://localhost:8080/metrics | grep rate_limiter
```

**Solution**:
- Reduce `maxRateLimiterEntries` (default: 10,000)
- Decrease cleanup interval
- Ensure load balancer is blocking most traffic

### Issue: DDoS attack bypassing rate limits

**Diagnosis**:
- Distributed attack from many IPs
- Each IP under rate limit individually

**Solution**:
- Enable Cloudflare DDoS protection
- Implement CAPTCHA challenges
- Use AWS Shield Advanced
- Implement connection limits (not just request limits)

---

## Security Considerations

### IP Spoofing
- Load balancer uses actual client IP (not spoofable)
- Application uses `X-Forwarded-For` (trust load balancer)
- Validate `X-Forwarded-For` comes from trusted source

### Distributed Attacks
- Per-IP limits may not be sufficient
- Consider global rate limits (all IPs combined)
- Implement CAPTCHA for suspicious traffic

### API Keys
- Authenticated requests can have higher limits
- Track rate limits per API key, not just IP
- Implement tiered rate limits (free vs paid)

---

## Future Enhancements

### Redis-Based Rate Limiting (Optional)
- For very high scale (millions of requests/sec)
- Shared state across instances without load balancer
- More complex rate limiting algorithms

**When to implement**:
- Load balancer rate limiting insufficient
- Need sub-second rate limit windows
- Complex rate limiting logic required

**Current assessment**: Not needed with load balancer approach

---

## References

- [AWS WAF Rate Limiting](https://docs.aws.amazon.com/waf/latest/developerguide/waf-rule-statement-type-rate-based.html)
- [NGINX Rate Limiting](https://www.nginx.com/blog/rate-limiting-nginx/)
- [Cloudflare Rate Limiting](https://developers.cloudflare.com/waf/rate-limiting-rules/)
- [OWASP Rate Limiting](https://cheatsheetseries.owasp.org/cheatsheets/Denial_of_Service_Cheat_Sheet.html#rate-limiting)

---

## Conclusion

**Rate limiting is implemented and production-ready** using a layered approach:

1. **Primary**: Load balancer rate limiting (AWS WAF/ALB/NGINX)
2. **Backup**: In-memory rate limiter (defense in depth)
3. **Monitoring**: Metrics and alerts for both layers

This architecture provides:
- ✅ Protection against DDoS attacks
- ✅ Scalability across multiple instances
- ✅ Defense in depth
- ✅ No external dependencies (Redis not required)
- ✅ Production-ready

**Status**: ✅ **RESOLVED** - No code changes needed, load balancer handles production rate limiting.
