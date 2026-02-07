---
name: performance-review
description: Identify performance bottlenecks and scalability issues
---

Perform a performance and scalability review.

Specify the component to review and target metrics (requests/second, latency targets, concurrent users, uptime requirements).

### ANALYSIS AREAS:

#### 1. DATABASE:
- [ ] Query efficiency (EXPLAIN ANALYZE)
- [ ] Missing indexes
- [ ] N+1 queries
- [ ] Connection pooling
- [ ] Query timeouts
- [ ] Transaction isolation levels
- [ ] Batch operations

#### 2. MEMORY:
- [ ] Unbounded data structures
- [ ] Memory leaks
- [ ] Large allocations
- [ ] Caching strategy
- [ ] GC pressure
- [ ] Memory limits

#### 3. CPU:
- [ ] Inefficient algorithms
- [ ] Unnecessary computations
- [ ] Blocking operations
- [ ] Goroutine management
- [ ] CPU-bound operations

#### 4. I/O:
- [ ] File I/O efficiency
- [ ] Network calls
- [ ] External API calls
- [ ] Serialization/deserialization
- [ ] Buffering

#### 5. CONCURRENCY:
- [ ] Lock contention
- [ ] Channel blocking
- [ ] Goroutine leaks
- [ ] Race conditions
- [ ] Deadlocks

#### 6. CACHING:
- [ ] Cache hit rate
- [ ] Cache invalidation
- [ ] Cache size limits
- [ ] Cache warming
- [ ] Distributed caching

### LOAD TESTING SCENARIOS:
1. **Normal load**: {{TARGET_RPS}} req/s for 1 hour
2. **Peak load**: {{PEAK_RPS}} req/s for 15 minutes
3. **Spike**: 0 to {{SPIKE_RPS}} req/s in 10 seconds
4. **Endurance**: {{TARGET_RPS}} req/s for {{UPTIME_DAYS}} days

### DELIVERABLE:
For each issue found:
1. **Bottleneck**: What's slow
2. **Impact**: How much slower (measurements)
3. **Fix**: Optimization with code
4. **Benchmark**: Before/after comparison

### CRITICAL REQUIREMENT:
Provide actual measurements, not guesses. Run benchmarks.
