package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDedupMiddleware_CachesResponse verifies duplicate requests return cached response.
func TestDedupMiddleware_CachesResponse(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("X-Call-Count", string(rune('0'+callCount)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	})

	dedup := NewDedupMiddleware(5 * time.Minute)
	wrapped := dedup.Wrap(handler)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	// First request
	req1, _ := http.NewRequest("POST", srv.URL, strings.NewReader("{}"))
	req1.Header.Set("X-Request-ID", "test-123")
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()

	if callCount != 1 {
		t.Errorf("Handler should be called once, got %d", callCount)
	}

	// Second request with same ID - should return cached
	req2, _ := http.NewRequest("POST", srv.URL, strings.NewReader("{}"))
	req2.Header.Set("X-Request-ID", "test-123")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	if callCount != 1 {
		t.Errorf("Handler should still be called once (cached), got %d", callCount)
	}

	if string(body1) != string(body2) {
		t.Errorf("Cached response should match: got %s vs %s", body1, body2)
	}
}

// TestDedupMiddleware_DifferentIDs verifies different request IDs execute separately.
func TestDedupMiddleware_DifferentIDs(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	dedup := NewDedupMiddleware(5 * time.Minute)
	wrapped := dedup.Wrap(handler)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	// Request with ID "a"
	req1, _ := http.NewRequest("POST", srv.URL, nil)
	req1.Header.Set("X-Request-ID", "a")
	http.DefaultClient.Do(req1)

	// Request with ID "b" - different ID, should execute
	req2, _ := http.NewRequest("POST", srv.URL, nil)
	req2.Header.Set("X-Request-ID", "b")
	http.DefaultClient.Do(req2)

	if callCount != 2 {
		t.Errorf("Handler should be called twice for different IDs, got %d", callCount)
	}
}

// TestDedupMiddleware_NoHeader verifies requests without X-Request-ID execute normally.
func TestDedupMiddleware_NoHeader(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	dedup := NewDedupMiddleware(5 * time.Minute)
	wrapped := dedup.Wrap(handler)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	// Two requests without X-Request-ID
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("POST", srv.URL, nil)
		http.DefaultClient.Do(req)
	}

	if callCount != 2 {
		t.Errorf("Handler should be called twice without dedup header, got %d", callCount)
	}
}

// TestDedupMiddleware_GETNotCached verifies GET requests bypass dedup.
func TestDedupMiddleware_GETNotCached(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	dedup := NewDedupMiddleware(5 * time.Minute)
	wrapped := dedup.Wrap(handler)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	// Two GET requests with same ID - both should execute
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL, nil)
		req.Header.Set("X-Request-ID", "same-id")
		http.DefaultClient.Do(req)
	}

	if callCount != 2 {
		t.Errorf("GET requests should not be cached, got %d calls", callCount)
	}
}

// TestDedupMiddleware_Expiry verifies cache entries expire after TTL.
func TestDedupMiddleware_Expiry(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	// Short TTL for testing
	dedup := NewDedupMiddleware(50 * time.Millisecond)
	wrapped := dedup.Wrap(handler)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	// First request
	req1, _ := http.NewRequest("POST", srv.URL, nil)
	req1.Header.Set("X-Request-ID", "expire-test")
	http.DefaultClient.Do(req1)

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Second request after expiry - should execute again
	req2, _ := http.NewRequest("POST", srv.URL, nil)
	req2.Header.Set("X-Request-ID", "expire-test")
	http.DefaultClient.Do(req2)

	if callCount != 2 {
		t.Errorf("Handler should be called twice after expiry, got %d", callCount)
	}
}
