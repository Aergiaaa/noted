package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// --- WriteError ---

func TestWriteError_envelopeShapePerStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		message string
		fields  map[string]string
		code    string
		wantMsg string
	}{
		{"400 passes message and fields", 400, "title is required",
			map[string]string{"title": "required"}, "validation_error", "title is required"},
		{"401 defaults message", 401, "", nil, "unauthenticated", "unauthenticated"},
		{"403 defaults message", 403, "", nil, "origin_forbidden", "forbidden origin"},
		{"404 defaults message", 404, "", nil, "not_found", "not found"},
		{"409 defaults message", 409, "", nil, "conflict", "conflict"},
		{"413 defaults message", 413, "", nil, "payload_too_large", "request body too large"},
		{"429 defaults message", 429, "", nil, "rate_limited", "too many requests"},
		{"500 never leaks details", 500, "", nil, "internal_error", "internal server error"},
		{"unknown status falls back", 418, "", nil, "error", "request failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			WriteError(rec, tt.status, tt.message, tt.fields)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			var body apiError
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Code != tt.code {
				t.Errorf("code = %q, want %q", body.Code, tt.code)
			}
			if body.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", body.Message, tt.wantMsg)
			}
			if tt.fields == nil {
				if body.Fields != nil {
					t.Errorf("fields = %v, want omitted", body.Fields)
				}
			} else if !reflect.DeepEqual(body.Fields, tt.fields) {
				t.Errorf("fields = %v, want %v", body.Fields, tt.fields)
			}
		})
	}
}

func TestWriteError_explicitMessageOverridesDefault(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusForbidden, "forbidden origin", nil)

	var body apiError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != "forbidden origin" {
		t.Fatalf("message = %v, want %q", body.Message, "forbidden origin")
	}
}

// --- DecodeJSON ---

func TestDecodeJSON_validBody_decodesAndReturnsTrue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/notes",
		strings.NewReader(`{"title":"hello"}`))
	rec := httptest.NewRecorder()
	var dst struct {
		Title string `json:"title"`
	}

	ok := DecodeJSON(rec, req, &dst)

	if !ok {
		t.Fatalf("DecodeJSON = false (status %d), want true", rec.Code)
	}
	if dst.Title != "hello" {
		t.Fatalf("title = %q, want %q", dst.Title, "hello")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (nothing written)", rec.Code)
	}
}

func TestDecodeJSON_malformedBody_400Envelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"title":`))
	rec := httptest.NewRecorder()

	if DecodeJSON(rec, req, &struct{}{}) {
		t.Fatal("DecodeJSON = true, want false")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "validation_error")
}

func TestDecodeJSON_emptyBody_400Envelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(""))
	rec := httptest.NewRecorder()

	if DecodeJSON(rec, req, &struct{}{}) {
		t.Fatal("DecodeJSON = true, want false")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "validation_error")
}

func TestDecodeJSON_oversizedBody_413Envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	// Valid JSON over the limit: the decoder must read past the cap to
	// finish the object, so MaxBytesReader trips mid-parse (a malformed
	// oversized body would surface as a 400 syntax error first).
	req := httptest.NewRequest(http.MethodPost, "/api/notes",
		strings.NewReader(`{"title":"`+strings.Repeat("x", 64)+`"}`))
	req.Body = http.MaxBytesReader(rec, req.Body, 8)

	if DecodeJSON(rec, req, &struct{}{}) {
		t.Fatal("DecodeJSON = true, want false")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	assertErrorCode(t, rec, "payload_too_large")
}

// assertErrorCode decodes the envelope and checks its error code.
func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env apiError
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	if env.Code != want {
		t.Fatalf("code = %q, want %q (body %q)", env.Code, want, body)
	}
}
