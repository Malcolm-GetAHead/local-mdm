package api

import (
	"container/list"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/constants"
)

// Simple in-memory rate limiter with LRU eviction
type rateLimiter struct {
	requests map[string][]time.Time
	lru      *list.List
	lruMap   map[string]*list.Element
	mu       sync.RWMutex
	limit    int
	window   time.Duration
	maxSize  int
	stopChan chan struct{}
	stopped  bool
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return newRateLimiterWithSize(limit, window, constants.MaxRateLimiterEntries)
}

func newRateLimiterWithSize(limit int, window time.Duration, maxSize int) *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		lru:      list.New(),
		lruMap:   make(map[string]*list.Element),
		limit:    limit,
		window:   window,
		maxSize:  maxSize,
		stopChan: make(chan struct{}),
	}
	
	// Cleanup old entries every minute
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-rl.stopChan:
				return
			}
		}
	}()
	
	return rl
}

// Stop stops the background cleanup goroutine
func (rl *rateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if !rl.stopped {
		close(rl.stopChan)
		rl.stopped = true
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-rl.window)
	
	// Evict oldest entry if at capacity and key doesn't exist
	if _, exists := rl.requests[key]; !exists && len(rl.requests) >= rl.maxSize {
		rl.evictOldest()
	}
	
	// Update LRU
	if element, ok := rl.lruMap[key]; ok {
		rl.lru.MoveToFront(element)
	} else {
		element := rl.lru.PushFront(key)
		rl.lruMap[key] = element
	}
	
	// Get existing requests for this key
	requests := rl.requests[key]
	
	// Filter out old requests
	var recent []time.Time
	for _, t := range requests {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	
	// Check if limit exceeded
	if len(recent) >= rl.limit {
		rl.requests[key] = recent
		return false
	}
	
	// Add new request
	recent = append(recent, now)
	rl.requests[key] = recent
	
	return true
}

func (rl *rateLimiter) evictOldest() {
	if rl.lru.Len() == 0 {
		return
	}
	
	oldest := rl.lru.Back()
	if oldest != nil {
		key := oldest.Value.(string)
		rl.lru.Remove(oldest)
		delete(rl.lruMap, key)
		delete(rl.requests, key)
	}
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-rl.window * 2)
	
	for key, requests := range rl.requests {
		var recent []time.Time
		for _, t := range requests {
			if t.After(cutoff) {
				recent = append(recent, t)
			}
		}
		
		if len(recent) == 0 {
			delete(rl.requests, key)
			if element, ok := rl.lruMap[key]; ok {
				rl.lru.Remove(element)
				delete(rl.lruMap, key)
			}
		} else {
			rl.requests[key] = recent
		}
	}
}

func rateLimitMiddleware(limiter *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Exempt device protocol endpoints — OMA-DM sends many messages per sync
			if strings.HasPrefix(r.URL.Path, "/ManagementServer/") ||
				strings.HasPrefix(r.URL.Path, "/EnrollmentServer/") ||
				r.URL.Path == "/scep" ||
				r.URL.Path == "/checkin" ||
				r.URL.Path == "/mdm" {
				next.ServeHTTP(w, r)
				return
			}

			// Use client IP — trust X-Forwarded-For from ALB
			key := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				if parts := strings.SplitN(xff, ",", 2); len(parts) > 0 {
					key = strings.TrimSpace(parts[0])
				}
			}
			
			if !limiter.allow(key) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}
