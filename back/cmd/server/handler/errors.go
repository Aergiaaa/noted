package handler

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Error is the inner object of the frozen error envelope (API.md): every
// non-2xx response body is exactly {"error": {code, message, fields?}} so
// front/lib/api.ts (F5) can parse one shape for all statuses.
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// apiError wraps Error under the envelope's "error" key — the wire shape
// is a single object with one nested member, so the struct stays two
// lines and callers build one literal.
type apiError struct {
	Error `json:"error"`
}

// errorCodes maps every status the API may return to its frozen code and
// the default (generic, detail-free) message. Callers override the message
// only when it is safe to show; 500 always uses the default so internals
// never leak to the client (API.md).
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

// WriteError writes the error envelope for status. An empty message picks
// the generic default from errorCodes; fields is optional per-field
// validation detail (API.md "Validation errors").
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

// WriteJSON writes the success body for status: one JSON object (API.md
// success shapes). Errors have their own WriteError above so no caller
// can invent a second failure format.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON reads one JSON object from the request body into dst and
// reports whether it succeeded. On failure it has already written the
// envelope: 413 when middleware.BodyLimit tripped (MaxBytesReader), 400
// otherwise. Handlers use it as a guard: if !DecodeJSON(w, r, &in) { return }.
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
