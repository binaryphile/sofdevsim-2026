package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDecodeJSON verifies JSON decoding logic.
// Pure calculation - returns error without writing HTTP response.
func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{
			name:    "valid JSON",
			body:    `{"name":"Alice","age":30}`,
			wantErr: nil,
		},
		{
			name:    "invalid JSON",
			body:    `{"name": invalid}`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "empty body",
			body:    "",
			wantErr: ErrInvalidJSON,
		},
		{
			name:    "malformed JSON",
			body:    `{"name":`,
			wantErr: ErrInvalidJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))

			var dest payload
			err := decodeJSON(req, &dest)

			if tt.wantErr == nil && err != nil {
				t.Errorf("decodeJSON() unexpected error = %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("decodeJSON() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestDecodeJSON_MaxBytesError verifies body-too-large detection.
func TestDecodeJSON_MaxBytesError(t *testing.T) {
	// Create a request with MaxBytesReader limiting to 10 bytes
	largeBody := strings.NewReader(`{"name":"this is way too long"}`)
	req := httptest.NewRequest("POST", "/test", largeBody)

	// Wrap with MaxBytesReader (simulates LimitBody middleware)
	w := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(w, req.Body, 10)

	type payload struct {
		Name string `json:"name"`
	}
	var dest payload
	err := decodeJSON(req, &dest)

	if !errors.Is(err, ErrBodyTooLarge) {
		t.Errorf("decodeJSON() error = %v, want %v", err, ErrBodyTooLarge)
	}
}

// TestRespondDecodeError verifies HTTP status code mapping.
// Tests both bare sentinel errors and wrapped errors to verify errors.Is() behavior.
func TestRespondDecodeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "body too large returns 413",
			err:        ErrBodyTooLarge,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "wrapped body too large returns 413",
			err:        fmt.Errorf("reading request: %w", ErrBodyTooLarge),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "invalid JSON returns 400",
			err:        ErrInvalidJSON,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrapped invalid JSON returns 400",
			err:        fmt.Errorf("parsing body: %w", ErrInvalidJSON),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "other error returns 400",
			err:        errors.New("some other error"),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondDecodeError(w, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("respondDecodeError() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// limitedReader simulates a reader that returns MaxBytesError.
type limitedReader struct {
	err error
}

func (r *limitedReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func (r *limitedReader) Close() error {
	return nil
}

// Ensure io.ReadCloser is implemented
var _ io.ReadCloser = (*limitedReader)(nil)
