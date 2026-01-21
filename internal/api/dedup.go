package api

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// DedupMiddleware caches responses by X-Request-ID header.
// Duplicate requests with same ID return cached response without re-execution.
// Cache entries expire after TTL (default 5 minutes).
type DedupMiddleware struct {
	cache map[string]cachedResponse
	mu    sync.RWMutex
	ttl   time.Duration
}

type cachedResponse struct {
	status  int
	headers http.Header
	body    []byte
	expires time.Time
}

// NewDedupMiddleware creates a new deduplication middleware with the given TTL.
func NewDedupMiddleware(ttl time.Duration) *DedupMiddleware {
	d := &DedupMiddleware{
		cache: make(map[string]cachedResponse),
		ttl:   ttl,
	}
	// Start cleanup goroutine
	go d.cleanupLoop()
	return d
}

// Wrap wraps an http.Handler with deduplication logic.
// Only applies to POST requests with X-Request-ID header.
func (d *DedupMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only deduplicate POST requests
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// No request ID, execute normally
			next.ServeHTTP(w, r)
			return
		}

		// Check cache
		d.mu.RLock()
		cached, found := d.cache[requestID]
		d.mu.RUnlock()

		if found && time.Now().Before(cached.expires) {
			// Return cached response
			for k, v := range cached.headers {
				w.Header()[k] = v
			}
			w.WriteHeader(cached.status)
			w.Write(cached.body)
			return
		}

		// Execute and cache
		recorder := &responseRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
			body:           &bytes.Buffer{},
		}
		next.ServeHTTP(recorder, r)

		// Cache the response
		d.mu.Lock()
		d.cache[requestID] = cachedResponse{
			status:  recorder.status,
			headers: recorder.Header().Clone(),
			body:    recorder.body.Bytes(),
			expires: time.Now().Add(d.ttl),
		}
		d.mu.Unlock()
	})
}

// responseRecorder captures the response for caching.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// cleanupLoop periodically removes expired cache entries.
func (d *DedupMiddleware) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for id, cached := range d.cache {
			if now.After(cached.expires) {
				delete(d.cache, id)
			}
		}
		d.mu.Unlock()
	}
}
