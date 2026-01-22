package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequireJSON verifies Content-Type validation middleware.
func TestRequireJSON(t *testing.T) {
	// Handler that records if it was called
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireJSON(inner)

	tests := []struct {
		name         string
		method       string
		contentType  string
		wantStatus   int
		wantCalled   bool
	}{
		{
			name:        "GET bypasses check",
			method:      "GET",
			contentType: "",
			wantStatus:  http.StatusOK,
			wantCalled:  true,
		},
		{
			name:        "DELETE bypasses check",
			method:      "DELETE",
			contentType: "",
			wantStatus:  http.StatusOK,
			wantCalled:  true,
		},
		{
			name:        "POST with application/json passes",
			method:      "POST",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			wantCalled:  true,
		},
		{
			name:        "POST with charset passes",
			method:      "POST",
			contentType: "application/json; charset=utf-8",
			wantStatus:  http.StatusOK,
			wantCalled:  true,
		},
		{
			name:        "POST without Content-Type returns 415",
			method:      "POST",
			contentType: "",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantCalled:  false,
		},
		{
			name:        "POST with text/plain returns 415",
			method:      "POST",
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantCalled:  false,
		},
		{
			name:        "PATCH with application/json passes",
			method:      "PATCH",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			wantCalled:  true,
		},
		{
			name:        "PATCH without Content-Type returns 415",
			method:      "PATCH",
			contentType: "",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantCalled:  false,
		},
		{
			name:        "PUT with application/json passes",
			method:      "PUT",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			wantCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}
