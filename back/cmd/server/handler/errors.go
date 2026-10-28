package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"noted/internal/service"
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

	WriteErrorCode(w, status, e.code, message, fields)
}

// WriteErrorCode writes the envelope with an explicit code, for the one
// status whose default code does not fit: 403 maps to origin_forbidden,
// so the recovery-rotation step-up answers 403 step_up_required (API.md).
func WriteErrorCode(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	body := apiError{Error: Error{Code: code, Message: message, Fields: fields}}

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
// 413 when middleware.BodyLimit tripped (MaxBytesError), 400 otherwise —
// including unknown request fields (DisallowUnknownFields), so a typo'd
// key in the frontend request fails loudly instead of being dropped.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
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

// writeServiceError maps service failures onto the frozen envelope:
// ValidationError → 400 with its fields, ErrNotFound → 404, anything
// else → a generic 500 with the detail in the server log only (API.md).
func writeServiceError(w http.ResponseWriter, err error, op string) {
	var validation *service.ValidationError

	switch {
	case errors.As(err, &validation):
		WriteError(w, http.StatusBadRequest, "", validation.Fields)
	case errors.Is(err, service.ErrNotFound):
		WriteError(w, http.StatusNotFound, "", nil)
	default:
		slog.Error("handler", "op", op, "err", err)
		WriteError(w, http.StatusInternalServerError, "", nil)
	}
}

// writeFieldError is the single-field validation_error case (API.md).
func writeFieldError(w http.ResponseWriter, field, message string) {
	WriteError(w, http.StatusBadRequest, "", map[string]string{field: message})
}
