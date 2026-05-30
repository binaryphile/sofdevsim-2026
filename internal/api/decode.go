package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Sentinel errors for decode failures (Data per FP guide).
var (
	ErrBodyTooLarge = errors.New("request body too large")
	ErrInvalidJSON  = errors.New("invalid JSON")
)

// decodeJSON decodes JSON from request body. Pure calculation - returns error, no I/O.
func decodeJSON[T any](r *http.Request, dest *T) error {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return ErrBodyTooLarge
		}
		return ErrInvalidJSON
	}
	return nil
}

// respondDecodeError was removed in the #18915 fluentfp/web migration.
// Use decodeError(err) in adapt.go, which returns a typed *web.Error that
// callers wrap with rslt.Err[web.Response](...). The body-too-large/invalid-JSON
// distinction (413 vs 400) is preserved in the new helper.
