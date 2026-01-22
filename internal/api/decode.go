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

// respondDecodeError writes appropriate HTTP error for decode failures.
func respondDecodeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrBodyTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
