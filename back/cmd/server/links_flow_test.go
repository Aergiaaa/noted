package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"noted/internal/auth"
)

// TestLinkFlow_endToEnd walks the F9 critical path through the real
// router: a linked note/event/task/transaction trio, then an event
// soft-delete. Peers keep their links until purge (DOMAIN.md), the event
// leaves the live list while trash still lists it, and edits/unlinks keep
// working — a stored link to the trashed peer never re-fails validation
// (API.md PATCH: absent keeps; "" clears).
func TestLinkFlow_endToEnd(t *testing.T) {
	app := newTestApp(t, Config{TOTPEncKey: cliEncKey, CurrencyDefault: "IDR"})
	router := app.newRouter()

	secret, _ := hostEnroll(t, app)
	live, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	login := postAuth(t, router, "/api/auth/login", `{"code":"`+live+`"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200", login.Code)
	}
	cookie := sessionCookie(t, login)

	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: cookie})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	create := func(path, body string) string {
		t.Helper()
		rec := call(http.MethodPost, path, body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST %s = %d, want 201: %s", path, rec.Code, rec.Body.String())
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return out.ID
	}

	noteID := create("/api/notes", `{"title":"Keyboard research"}`)
	eventID := create("/api/events",
		`{"title":"Buy keyboard","starts_at":"2026-09-20T10:00:00Z",`+
			`"ends_at":"2026-09-20T11:00:00Z","note_id":"`+noteID+`"}`)
	taskID := create("/api/tasks",
		`{"title":"Buy keyboard","due_date":"2026-09-20",`+
			`"event_id":"`+eventID+`","note_id":"`+noteID+`"}`)
	txID := create("/api/finance/transactions",
		`{"kind":"expense","wallet_id":"wal_cash","amount_cents":500000,`+
			`"occurred_on":"2026-09-20","task_id":"`+taskID+`",`+
			`"event_id":"`+eventID+`","note_id":"`+noteID+`"}`)

	if rec := call(http.MethodDelete, "/api/events/"+eventID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE event = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	// The trio survives the soft-delete with links intact.
	getJSON := func(path string, out any) {
		t.Helper()
		rec := call(http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
		if err := json.NewDecoder(rec.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}

	var tasks struct {
		Data []struct {
			ID      string  `json:"id"`
			EventID *string `json:"event_id"`
			NoteID  *string `json:"note_id"`
		} `json:"data"`
	}
	getJSON("/api/tasks?status=all", &tasks)
	if len(tasks.Data) != 1 {
		t.Fatalf("tasks = %d rows, want 1", len(tasks.Data))
	}
	kept := tasks.Data[0]
	if kept.EventID == nil || *kept.EventID != eventID {
		t.Errorf("task.event_id = %v, want kept %s", kept.EventID, eventID)
	}
	if kept.NoteID == nil || *kept.NoteID != noteID {
		t.Errorf("task.note_id = %v, want kept %s", kept.NoteID, noteID)
	}

	var page struct {
		Data []struct {
			ID      string  `json:"id"`
			TaskID  *string `json:"task_id"`
			EventID *string `json:"event_id"`
			NoteID  *string `json:"note_id"`
		} `json:"data"`
	}
	getJSON("/api/finance/transactions", &page)
	if len(page.Data) != 1 {
		t.Fatalf("transactions = %d rows, want 1", len(page.Data))
	}
	tx := page.Data[0]
	if tx.TaskID == nil || *tx.TaskID != taskID {
		t.Errorf("transaction.task_id = %v, want kept %s", tx.TaskID, taskID)
	}
	if tx.EventID == nil || *tx.EventID != eventID {
		t.Errorf("transaction.event_id = %v, want kept %s", tx.EventID, eventID)
	}
	if tx.NoteID == nil || *tx.NoteID != noteID {
		t.Errorf("transaction.note_id = %v, want kept %s", tx.NoteID, noteID)
	}

	// The event leaves the live list; trash still lists it.
	var events struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	getJSON("/api/events?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z", &events)
	if len(events.Data) != 0 {
		t.Errorf("live events = %d rows, want 0 after delete", len(events.Data))
	}
	getJSON("/api/events?trash=1", &events)
	if len(events.Data) != 1 || events.Data[0].ID != eventID {
		t.Errorf("trash events = %+v, want [%s]", events.Data, eventID)
	}

	// Edits keep working with the trashed peer: TaskForm omits the link
	// keys (absent = keep), and the chip unlink clears one field only.
	var edited struct {
		ID      string  `json:"id"`
		EventID *string `json:"event_id"`
		NoteID  *string `json:"note_id"`
	}
	rec := call(http.MethodPatch, "/api/tasks/"+taskID, `{"title":"Buy keyboard tomorrow"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH task = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if err := json.NewDecoder(rec.Body).Decode(&edited); err != nil {
		t.Fatalf("decode patched task: %v", err)
	}
	if edited.EventID == nil || *edited.EventID != eventID {
		t.Errorf("patched task.event_id = %v, want kept %s", edited.EventID, eventID)
	}

	rec = call(http.MethodPatch, "/api/finance/transactions/"+txID, `{"task_id":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH transaction = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var unlinked struct {
		TaskID  *string `json:"task_id"`
		EventID *string `json:"event_id"`
		NoteID  *string `json:"note_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&unlinked); err != nil {
		t.Fatalf("decode patched transaction: %v", err)
	}
	if unlinked.TaskID != nil {
		t.Errorf("transaction.task_id = %v, want cleared", unlinked.TaskID)
	}
	if unlinked.EventID == nil || *unlinked.EventID != eventID {
		t.Errorf("transaction.event_id = %v, want kept %s", unlinked.EventID, eventID)
	}
	if unlinked.NoteID == nil || *unlinked.NoteID != noteID {
		t.Errorf("transaction.note_id = %v, want kept %s", unlinked.NoteID, noteID)
	}
}
