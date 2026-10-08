package handler

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Error is the frozen error envelope inner object (API.md).
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type apiError struct {
	Error `json:"error"`
}

// errorCodes: 500 always keeps the generic default, so internals never
// leak to the client.
var errorCodes = map[int]struct{ code, message string }{
	http.StatusBadRequest:            {"validation_error", "validation failed"},
	http.StatusUnauthorized:          {"unauthenticated", "unauthenticated"},
	http.StatusForbidden:             {"origin_forbidden", "forbidden origin"},
	http.StatusNotFound:              {"not_found", "not found"},
	http.StatusConflict:              {"conflict", "conflict"},
	http.StatusRequestEntityTooLarge: {"payload_too_large", "request body too large"},
	http.StatusTooManyRequests:       {"rate_limited", "too many requests"},
	http.StatusInternalServerError:   {"internal_error", "internal server error"},
}

func WriteError(w http.ResponseWriter, status int, message string, fields map[string]string) {
	e, ok := errorCodes[status]
	if !ok {
		e = struct{ code, message string }{"error", "request failed"}
	}
	if message == "" {
		message = e.message
	}

	body := apiError{Error: Error{Code: e.code, Message: message, Fields: fields}}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON has already written the envelope when it reports failure:
// 413 when middleware.BodyLimit tripped (MaxBytesError), 400 otherwise.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteError(w, http.StatusRequestEntityTooLarge, "", nil)
		} else {
			WriteError(w, http.StatusBadRequest, "invalid JSON body", nil)
		}
		return false
	}
	return true
}
