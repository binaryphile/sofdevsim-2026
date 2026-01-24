package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

// BenchmarkDedup_CacheHit measures latency for returning cached response.
// Target: < 1μs (map lookup + response copy).
func BenchmarkDedup_CacheHit(b *testing.B) {
	dedup := api.NewDedupMiddleware(5 * time.Minute)

	// Handler that returns a small JSON response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := dedup.Wrap(handler)

	// Prime the cache with first request
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Request-ID", "bench-hit-id")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-Request-ID", "bench-hit-id")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkDedup_CacheMiss measures overhead of caching a new response.
// Target: handler time + < 10μs overhead.
func BenchmarkDedup_CacheMiss(b *testing.B) {
	dedup := api.NewDedupMiddleware(5 * time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := dedup.Wrap(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		// Unique ID per iteration = cache miss every time
		req.Header.Set("X-Request-ID", string(rune(i)))
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkDedup_NoHeader measures pass-through latency when no X-Request-ID.
// Should be minimal overhead.
func BenchmarkDedup_NoHeader(b *testing.B) {
	dedup := api.NewDedupMiddleware(5 * time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := dedup.Wrap(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		// No X-Request-ID header
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkDedup_Contention measures lock contention under concurrent access.
func BenchmarkDedup_Contention(b *testing.B) {
	dedup := api.NewDedupMiddleware(5 * time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := dedup.Wrap(handler)

	// Prime cache
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-Request-ID", string(rune(i)))
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("X-Request-ID", string(rune(i%100))) // Hit existing cache entries
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)
			i++
		}
	})
}

// BenchmarkDedup_LargeResponse measures memory overhead for large cached responses.
func BenchmarkDedup_LargeResponse(b *testing.B) {
	dedup := api.NewDedupMiddleware(5 * time.Minute)

	// 100KB response body
	largeBody := bytes.Repeat([]byte("x"), 100*1024)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(largeBody)
	})

	wrapped := dedup.Wrap(handler)

	// Prime cache
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Request-ID", "large-response-id")
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-Request-ID", "large-response-id")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

// BenchmarkDedup_MemoryGrowth measures allocations as cache grows.
func BenchmarkDedup_MemoryGrowth(b *testing.B) {
	dedup := api.NewDedupMiddleware(5 * time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := dedup.Wrap(handler)

	var mu sync.Mutex
	idCounter := 0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		id := idCounter
		idCounter++
		mu.Unlock()

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-Request-ID", string(rune(id)))
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}
