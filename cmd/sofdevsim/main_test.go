package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServerRunning verifies service discovery via /health endpoint.
// Uses httptest for external HTTP boundary per Khorikov controller testing.
func TestServerRunning(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    bool
	}{
		{
			name: "sofdevsim server returns true",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"service": "sofdevsim", "version": "1.0"})
			},
			want: true,
		},
		{
			name: "wrong service returns false",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"service": "other-service"})
			},
			want: false,
		},
		{
			name: "invalid JSON returns false",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("not json"))
			},
			want: false,
		},
		{
			name: "server error returns false",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			got := serverRunning(srv.URL)
			if got != tt.want {
				t.Errorf("serverRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestServerRunning_NoServer verifies behavior when no server is running.
func TestServerRunning_NoServer(t *testing.T) {
	got := serverRunning("http://localhost:59999")
	if got != false {
		t.Errorf("serverRunning() with no server = %v, want false", got)
	}
}
