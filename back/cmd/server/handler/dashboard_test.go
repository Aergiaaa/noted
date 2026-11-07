package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"noted/internal/database"
	"noted/internal/database/generated"
	"noted/internal/service"
)

func newTestDashboardHandler(t *testing.T) (*Handler, http.Handler, *service.DashboardService, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "handler-dashboard.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	q := generated.New(db)
	events := service.NewEventService(q)
	notes := service.NewNoteService(q)
	tasks := service.NewTaskService(q)
	transactions := service.NewTransactionService(q, "USD")
	dashboardSvc := service.NewDashboardService(events, notes, tasks, transactions)
	h := New(nil, events, notes, tasks, nil, transactions, dashboardSvc, false)

	r := chi.NewRouter()
	r.Get("/api/dashboard", h.GetDashboard)
	return h, r, dashboardSvc, db
}

func dashRequest(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func TestGetDashboard_returns200AndComposedData(t *testing.T) {
	_, router, _, db := newTestDashboardHandler(t)
	q := generated.New(db)
	events := service.NewEventService(q)
	notes := service.NewNoteService(q)
	tasks := service.NewTaskService(q)
	transactions := service.NewTransactionService(q, "USD")

	ctx := context.Background()
	today := "2026-10-29"

	// Create test data via services
	_, err := events.Create(ctx, service.EventInput{
		Title:       "Team meeting",
		StartsAt:    mustTime(t, today+"T10:00:00Z"),
		EndsAt:      mustTime(t, today+"T11:00:00Z"),
		Description: "",
		NoteID:      "",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	_, err = tasks.Create(ctx, service.TaskInput{
		Title:   "Task today",
		Status:  "open",
		DueDate: today,
		EventID: "",
		NoteID:  "",
	})
	if err != nil {
		t.Fatalf("create task today: %v", err)
	}

	_, err = tasks.Create(ctx, service.TaskInput{
		Title:   "Overdue task",
		Status:  "open",
		DueDate: "2026-10-28",
		EventID: "",
		NoteID:  "",
	})
	if err != nil {
		t.Fatalf("create overdue task: %v", err)
	}

	// Create wallet for transactions
	wallets := service.NewWalletService(q)
	wallet, err := wallets.Create(ctx, "Cash")
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	memo := "Test expense"
	_, err = transactions.Create(ctx, service.TransactionInput{
		Kind:         service.KIND_EXPENSE,
		AmountCents:  1000,
		OccurredOn:   today,
		Memo:         memo,
		WalletID:     wallet.ID,
		FromWalletID: "",
		ToWalletID:   "",
		TaskID:       "",
		EventID:      "",
		NoteID:       "",
	})
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	_, err = notes.Create(ctx, service.NoteInput{
		Title:  "Pinned note",
		BodyMD: "Content",
		Pinned: true,
	})
	if err != nil {
		t.Fatalf("create pinned note: %v", err)
	}

	rec := dashRequest(t, router, http.MethodGet, "/api/dashboard?date="+today)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	// The response must carry the snake_case keys lib/api.ts pins — a
	// PascalCase row (StartsAt) reads as undefined on the page.
	var d struct {
		Date        string `json:"date"`
		EventsToday []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			StartsAt string `json:"starts_at"`
			EndsAt   string `json:"ends_at"`
		} `json:"events_today"`
		TasksToday []struct {
			ID      string  `json:"id"`
			Title   string  `json:"title"`
			Status  string  `json:"status"`
			DueDate *string `json:"due_date"`
		} `json:"tasks_today"`
		OverdueTasks []struct {
			Title string `json:"title"`
		} `json:"overdue_tasks"`
		RecentTransactions []struct {
			AmountCents int64   `json:"amount_cents"`
			Currency    string  `json:"currency"`
			OccurredOn  string  `json:"occurred_on"`
			Memo        *string `json:"memo"`
		} `json:"recent_transactions"`
		PinnedNotes []struct {
			Title  string `json:"title"`
			Pinned bool   `json:"pinned"`
		} `json:"pinned_notes"`
		Balance struct {
			Currency string `json:"currency"`
			Total    struct {
				BalanceCents int64 `json:"balance_cents"`
			} `json:"total"`
			Wallets []struct {
				WalletID     string `json:"wallet_id"`
				Name         string `json:"name"`
				BalanceCents int64  `json:"balance_cents"`
			} `json:"wallets"`
		} `json:"balance"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&d); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if d.Date != today {
		t.Fatalf("date = %q, want %q", d.Date, today)
	}
	if len(d.EventsToday) != 1 || d.EventsToday[0].Title != "Team meeting" {
		t.Fatalf("events_today = %+v", d.EventsToday)
	}
	if d.EventsToday[0].StartsAt != today+"T10:00:00Z" || d.EventsToday[0].EndsAt != today+"T11:00:00Z" {
		t.Fatalf("event times = %q/%q, want starts_at/ends_at keys", d.EventsToday[0].StartsAt, d.EventsToday[0].EndsAt)
	}
	if len(d.TasksToday) != 1 || d.TasksToday[0].Title != "Task today" {
		t.Fatalf("tasks_today = %+v", d.TasksToday)
	}
	if d.TasksToday[0].DueDate == nil || *d.TasksToday[0].DueDate != today || d.TasksToday[0].Status != "open" {
		t.Fatalf("task fields = %+v, want due_date/status keys", d.TasksToday[0])
	}
	if len(d.OverdueTasks) != 1 || d.OverdueTasks[0].Title != "Overdue task" {
		t.Fatalf("overdue_tasks = %+v", d.OverdueTasks)
	}
	if len(d.RecentTransactions) != 1 || d.RecentTransactions[0].Memo == nil || *d.RecentTransactions[0].Memo != "Test expense" {
		t.Fatalf("recent_transactions = %+v", d.RecentTransactions)
	}
	if d.RecentTransactions[0].AmountCents != 1000 || d.RecentTransactions[0].Currency != "USD" || d.RecentTransactions[0].OccurredOn != today {
		t.Fatalf("transaction fields = %+v, want amount_cents/currency/occurred_on keys", d.RecentTransactions[0])
	}
	if len(d.PinnedNotes) != 1 || d.PinnedNotes[0].Title != "Pinned note" || !d.PinnedNotes[0].Pinned {
		t.Fatalf("pinned_notes = %+v", d.PinnedNotes)
	}
	if d.Balance.Currency != "USD" {
		t.Fatalf("balance currency = %q, want USD", d.Balance.Currency)
	}
	if d.Balance.Total.BalanceCents != -1000 {
		t.Fatalf("balance total = %d, want -1000", d.Balance.Total.BalanceCents)
	}
	// The seeded wal_cash row is in the list too — assert the row for the
	// wallet we spent from, not the list length.
	var spent *struct {
		WalletID     string `json:"wallet_id"`
		Name         string `json:"name"`
		BalanceCents int64  `json:"balance_cents"`
	}
	for i := range d.Balance.Wallets {
		if d.Balance.Wallets[i].WalletID == wallet.ID {
			spent = &d.Balance.Wallets[i]
		}
	}
	if spent == nil {
		t.Fatalf("balance wallets = %+v, want a row for %q", d.Balance.Wallets, wallet.ID)
	}
	if spent.BalanceCents != -1000 {
		t.Fatalf("wallet balance = %d, want -1000", spent.BalanceCents)
	}
}

func TestGetDashboard_invalidDate_returns400(t *testing.T) {
	_, router, _, _ := newTestDashboardHandler(t)

	rec := dashRequest(t, router, http.MethodGet, "/api/dashboard?date=invalid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("code = %q, want validation_error", body.Error.Code)
	}
	if body.Error.Fields["date"] != "must be YYYY-MM-DD" {
		t.Fatalf("fields = %v", body.Error.Fields)
	}
}

func TestGetDashboard_defaultsToToday(t *testing.T) {
	_, router, _, _ := newTestDashboardHandler(t)

	rec := dashRequest(t, router, http.MethodGet, "/api/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var d struct {
		Date         string          `json:"date"`
		EventsToday  json.RawMessage `json:"events_today"`
		TasksToday   json.RawMessage `json:"tasks_today"`
		OverdueTasks json.RawMessage `json:"overdue_tasks"`
		RecentTx     json.RawMessage `json:"recent_transactions"`
		PinnedNotes  json.RawMessage `json:"pinned_notes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&d); err != nil {
		t.Fatalf("decode: %v", err)
	}

	expected := time.Now().Format("2006-01-02")
	if d.Date != expected {
		t.Fatalf("date = %q, want %q (today)", d.Date, expected)
	}

	// Empty cards must be [] — a null would crash the page's .filter().
	for name, raw := range map[string]json.RawMessage{
		"events_today":        d.EventsToday,
		"tasks_today":         d.TasksToday,
		"overdue_tasks":       d.OverdueTasks,
		"recent_transactions": d.RecentTx,
		"pinned_notes":        d.PinnedNotes,
	} {
		if string(raw) != "[]" {
			t.Fatalf("%s = %s, want []", name, raw)
		}
	}
}
