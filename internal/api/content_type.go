package api

import (
	"log/slog"
	"mime"
	"net/http"
)

// RequireJSON rejects requests without Content-Type: application/json.
// GET and DELETE requests bypass the check (no body expected).
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}
		ct := r.Header.Get("Content-Type")
		mediaType, _, _ := mime.ParseMediaType(ct)
		if mediaType != "application/json" {
			slog.Warn("rejected request: invalid content-type",
				"method", r.Method,
				"path", r.URL.Path,
				"content_type", ct,
				"remote_addr", r.RemoteAddr,
			)
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(w, r)
	})
}
