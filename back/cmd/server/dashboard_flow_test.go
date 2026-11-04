package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// TestDashboardFlow_endToEnd walks the F10 critical path through the real
// router: the DOMAIN.md "Buy keyboard" day (event + open task due that day
// + expense in Cash), an overdue task, and a pinned note beside an
// unpinned one — then GET /api/dashboard?date= must compose every card
// from the same row set, answer snake_case wire keys (starts_at is what
// getEventsToday filters on), and keep the F8 balance math (total =
// income − expense).
func TestDashboardFlow_endToEnd(t *testing.T) {
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

	create := func(path, body string) string {
		t.Helper()
		rec := postAuth(t, router, path, body, cookie)
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

	noteID := create("/api/notes", `{"title":"Keyboard research","pinned":true}`)
	create("/api/notes", `{"title":"Someday list"}`)
	eventID := create("/api/events",
		`{"title":"Buy keyboard","starts_at":"2026-09-20T10:00:00Z",`+
			`"ends_at":"2026-09-20T11:00:00Z"}`)
	taskID := create("/api/tasks",
		`{"title":"Buy keyboard","due_date":"2026-09-20","event_id":"`+eventID+`"}`)
	overdueID := create("/api/tasks", `{"title":"Buy cable","due_date":"2026-09-19"}`)
	create("/api/finance/transactions",
		`{"kind":"expense","wallet_id":"wal_cash","amount_cents":500000,`+
			`"occurred_on":"2026-09-20","memo":"mechanical",`+
			`"task_id":"`+taskID+`","event_id":"`+eventID+`","note_id":"`+noteID+`"}`)

	rec := getWithSession(t, router, "/api/dashboard?date=2026-09-20", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	// Every card list must be an array on the wire: a null would crash
	// DashboardPage's .filter() (the handler's nil→[] guarantee).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	for _, key := range []string{
		"events_today", "tasks_today", "overdue_tasks",
		"recent_transactions", "pinned_notes",
	} {
		if string(raw[key]) == "null" {
			t.Errorf("%s = null, want []", key)
		}
	}

	var d struct {
		Date string `json:"date"`
		// No omitempty games: a missing starts_at decodes as "" and
		// fails the exact-match below, which is the contract drift that
		// shipped the PascalCase crash.
		EventsToday []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			StartsAt string `json:"starts_at"`
			EndsAt   string `json:"ends_at"`
		} `json:"events_today"`
		TasksToday []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		} `json:"tasks_today"`
		OverdueTasks []struct {
			ID      string `json:"id"`
			DueDate string `json:"due_date"`
		} `json:"overdue_tasks"`
		RecentTransactions []struct {
			ID          string `json:"id"`
			AmountCents int64  `json:"amount_cents"`
			Memo        string `json:"memo"`
		} `json:"recent_transactions"`
		PinnedNotes []struct {
			ID     string `json:"id"`
			Pinned bool   `json:"pinned"`
		} `json:"pinned_notes"`
		Balance struct {
			Currency string `json:"currency"`
			Total    struct {
				BalanceCents int64 `json:"balance_cents"`
			} `json:"total"`
		} `json:"balance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}

	if d.Date != "2026-09-20" {
		t.Errorf("date = %q, want 2026-09-20", d.Date)
	}

	if len(d.EventsToday) != 1 {
		t.Fatalf("events_today = %d rows, want 1", len(d.EventsToday))
	}
	ev := d.EventsToday[0]
	if ev.ID != eventID || ev.Title != "Buy keyboard" {
		t.Errorf("events_today[0] = %s/%q, want %s/Buy keyboard", ev.ID, ev.Title, eventID)
	}
	if ev.StartsAt != "2026-09-20T10:00:00Z" || ev.EndsAt != "2026-09-20T11:00:00Z" {
		t.Errorf("event range = %q–%q, want 10:00–11:00Z", ev.StartsAt, ev.EndsAt)
	}

	if len(d.TasksToday) != 1 {
		t.Fatalf("tasks_today = %d rows, want 1", len(d.TasksToday))
	}
	if task := d.TasksToday[0]; task.ID != taskID || task.Status != "open" {
		t.Errorf("tasks_today[0] = %s/%q, want %s/open", task.ID, task.Status, taskID)
	}

	if len(d.OverdueTasks) != 1 {
		t.Fatalf("overdue_tasks = %d rows, want 1", len(d.OverdueTasks))
	}
	if task := d.OverdueTasks[0]; task.ID != overdueID || task.DueDate != "2026-09-19" {
		t.Errorf("overdue_tasks[0] = %s/%s, want %s/2026-09-19", task.ID, task.DueDate, overdueID)
	}

	if len(d.RecentTransactions) != 1 {
		t.Fatalf("recent_transactions = %d rows, want 1", len(d.RecentTransactions))
	}
	if tx := d.RecentTransactions[0]; tx.AmountCents != 500000 || tx.Memo != "mechanical" {
		t.Errorf("recent_transactions[0] = %d/%q, want 500000/mechanical", tx.AmountCents, tx.Memo)
	}

	// The unpinned note must not leak into the pinned card.
	if len(d.PinnedNotes) != 1 {
		t.Fatalf("pinned_notes = %d rows, want 1", len(d.PinnedNotes))
	}
	if note := d.PinnedNotes[0]; note.ID != noteID || !note.Pinned {
		t.Errorf("pinned_notes[0] = %s/%t, want %s/true", note.ID, note.Pinned, noteID)
	}

	if d.Balance.Currency != "IDR" {
		t.Errorf("currency = %q, want IDR (server-stamped)", d.Balance.Currency)
	}
	if d.Balance.Total.BalanceCents != -500000 {
		t.Errorf("total = %d, want -500000 (income − expense)", d.Balance.Total.BalanceCents)
	}
}
