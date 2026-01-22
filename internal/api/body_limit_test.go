package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLimitBody verifies body size limiting middleware.
func TestLimitBody(t *testing.T) {
	tests := []struct {
		name       string
		maxBytes   int64
		bodySize   int
		wantErr    bool
	}{
		{
			name:     "body within limit",
			maxBytes: 100,
			bodySize: 50,
			wantErr:  false,
		},
		{
			name:     "body exactly at limit",
			maxBytes: 100,
			bodySize: 100,
			wantErr:  false,
		},
		{
			name:     "body exceeds limit",
			maxBytes: 100,
			bodySize: 150,
			wantErr:  true,
		},
		{
			name:     "zero limit rejects any body",
			maxBytes: 0,
			bodySize: 1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var readErr error

			// Inner handler that reads the body
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, readErr = io.ReadAll(r.Body)
				if readErr != nil {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			handler := LimitBody(tt.maxBytes)(inner)

			body := strings.Repeat("x", tt.bodySize)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.wantErr && readErr == nil {
				t.Error("expected read error for oversized body, got nil")
			}
			if !tt.wantErr && readErr != nil {
				t.Errorf("unexpected read error: %v", readErr)
			}
		})
	}
}

// TestLimitBody_IntegrationWithDecodeJSON verifies middleware works with decodeJSON.
func TestLimitBody_IntegrationWithDecodeJSON(t *testing.T) {
	type payload struct {
		Data string `json:"data"`
	}

	// Limit to 50 bytes
	const maxBytes = 50

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if err := decodeJSON(r, &p); err != nil {
			respondDecodeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := LimitBody(maxBytes)(inner)

	t.Run("small body succeeds", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"data":"hi"}`))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("large body returns 413", func(t *testing.T) {
		// Create body larger than limit
		largeData := strings.Repeat("x", 100)
		body := `{"data":"` + largeData + `"}`
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
		}
	})
}
