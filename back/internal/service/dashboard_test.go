package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noted/internal/database"
	"noted/internal/database/generated"
)

// failingOverdueTaskServicer wraps TaskServicer and fails only on Overdue
// Used to test the Overdue error path in DashboardService.Get (line 65-67).
type failingOverdueTaskServicer struct {
	TaskServicer
}

func (f *failingOverdueTaskServicer) Overdue(ctx context.Context, day string) ([]Task, error) {
	return nil, errors.New("forced overdue error")
}

func TestDashboardService_Get_returnsComposedData(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	wallets := NewWalletService(q)
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	ctx := context.Background()

	// Create test data
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	// Event today
	_, err = events.Create(ctx, EventInput{
		Title:       "Team meeting",
		StartsAt:    mustTime(t, today+"T10:00:00Z"),
		EndsAt:      mustTime(t, today+"T11:00:00Z"),
		Description: "",
		NoteID:      "",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	// Task due today
	_, err = tasks.Create(ctx, TaskInput{
		Title:   "Task today",
		Status:  STATUS_OPEN,
		DueDate: today,
		EventID: "",
		NoteID:  "",
	})
	if err != nil {
		t.Fatalf("create task today: %v", err)
	}

	// Overdue task
	_, err = tasks.Create(ctx, TaskInput{
		Title:   "Overdue task",
		Status:  STATUS_OPEN,
		DueDate: yesterday,
		EventID: "",
		NoteID:  "",
	})
	if err != nil {
		t.Fatalf("create overdue task: %v", err)
	}

	// Create wallet for transactions
	wallet, err := wallets.Create(ctx, "Cash")
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	// Recent transaction
	memo := "Test expense"
	_, err = transactions.Create(ctx, TransactionInput{
		Kind:         KIND_EXPENSE,
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

	// Pinned note
	_, err = notes.Create(ctx, NoteInput{
		Title:  "Pinned note",
		BodyMD: "Content",
		Pinned: true,
	})
	if err != nil {
		t.Fatalf("create pinned note: %v", err)
	}

	// Call Get
	d, err := dashboard.Get(ctx, today)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Verify date
	if d.Date != today {
		t.Fatalf("date = %q, want %q", d.Date, today)
	}

	// Verify events today
	if len(d.EventsToday) != 1 {
		t.Fatalf("events today = %d, want 1", len(d.EventsToday))
	}
	if d.EventsToday[0].Title != "Team meeting" {
		t.Fatalf("event title = %q, want %q", d.EventsToday[0].Title, "Team meeting")
	}

	// Verify tasks today
	if len(d.TasksToday) != 1 {
		t.Fatalf("tasks today = %d, want 1", len(d.TasksToday))
	}
	if d.TasksToday[0].Title != "Task today" {
		t.Fatalf("task title = %q, want %q", d.TasksToday[0].Title, "Task today")
	}

	// Verify overdue tasks
	if len(d.OverdueTasks) != 1 {
		t.Fatalf("overdue tasks = %d, want 1", len(d.OverdueTasks))
	}
	if d.OverdueTasks[0].Title != "Overdue task" {
		t.Fatalf("overdue task title = %q, want %q", d.OverdueTasks[0].Title, "Overdue task")
	}

	// Verify recent transactions
	if len(d.RecentTransactions) != 1 {
		t.Fatalf("recent transactions = %d, want 1", len(d.RecentTransactions))
	}
	if d.RecentTransactions[0].Memo == nil || *d.RecentTransactions[0].Memo != "Test expense" {
		t.Fatalf("transaction memo = %v, want %q", d.RecentTransactions[0].Memo, "Test expense")
	}

	// Verify pinned notes
	if len(d.PinnedNotes) != 1 {
		t.Fatalf("pinned notes = %d, want 1", len(d.PinnedNotes))
	}
	if d.PinnedNotes[0].Title != "Pinned note" {
		t.Fatalf("pinned note title = %q, want %q", d.PinnedNotes[0].Title, "Pinned note")
	}

	// Verify balance
	if d.Balance.TotalCents != -1000 {
		t.Fatalf("balance total = %d, want -1000", d.Balance.TotalCents)
	}
}

func TestDashboardService_Get_invalidDate_returnsValidationError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard2.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "invalid-date")
	if err == nil {
		t.Fatal("expected validation error")
	}
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if valErr.Fields["date"] != "must be YYYY-MM-DD" {
		t.Fatalf("unexpected field error: %v", valErr.Fields)
	}
}

func TestDashboardService_Get_limitsRecentTransactionsAndPinnedNotes(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard3.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	wallets := NewWalletService(q)
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	// Create wallet for transactions
	wallet, err := wallets.Create(ctx, "Cash")
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	// Create 10 transactions
	for i := 0; i < 10; i++ {
		memo := "Tx " + string(rune('0'+i))
		_, err = transactions.Create(ctx, TransactionInput{
			Kind:         KIND_EXPENSE,
			AmountCents:  100,
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
			t.Fatalf("create tx %d: %v", i, err)
		}
	}

	// Create 10 pinned notes
	for i := 0; i < 10; i++ {
		_, err = notes.Create(ctx, NoteInput{
			Title:  "Note " + string(rune('0'+i)),
			BodyMD: "Content",
			Pinned: true,
		})
		if err != nil {
			t.Fatalf("create note %d: %v", i, err)
		}
	}

	d, err := dashboard.Get(ctx, today)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Should be limited to 5
	if len(d.RecentTransactions) != 5 {
		t.Fatalf("recent transactions = %d, want 5", len(d.RecentTransactions))
	}
	if len(d.PinnedNotes) != 5 {
		t.Fatalf("pinned notes = %d, want 5", len(d.PinnedNotes))
	}
}

func TestDashboardService_Get_eventsError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard4.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Close() // Close immediately to cause errors
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_Get_tasksError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard5.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Close()
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_Get_overdueError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard6.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Close()
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_Get_transactionsError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard7.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Close()
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_Get_notesError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard8.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Close()
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_Get_balanceError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard9.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Close()
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_Get_coverAllErrorPaths(t *testing.T) {
	// Test events error
	db, err := database.Open(filepath.Join(t.TempDir(), "dashboard10a.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS events"); err != nil {
		t.Fatalf("drop events table: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := generated.New(db)
	events := NewEventService(q)
	notes := NewNoteService(q)
	tasks := NewTaskService(q)
	transactions := NewTransactionService(q, "USD")
	dashboard := NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error from dropped events table")
	}

	// Test tasks error
	db, err = database.Open(filepath.Join(t.TempDir(), "dashboard10b.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS tasks"); err != nil {
		t.Fatalf("drop tasks table: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q = generated.New(db)
	events = NewEventService(q)
	notes = NewNoteService(q)
	tasks = NewTaskService(q)
	transactions = NewTransactionService(q, "USD")
	dashboard = NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error from dropped tasks table")
	}

	// Test overdue error path using a custom TaskServicer that fails only on Overdue
	// This covers line 65-67 in DashboardService.Get.
	db, err = database.Open(filepath.Join(t.TempDir(), "dashboard10c.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q = generated.New(db)
	events = NewEventService(q)
	notes = NewNoteService(q)
	realTasks := NewTaskService(q)
	customTasks := &failingOverdueTaskServicer{TaskServicer: realTasks}
	transactions = NewTransactionService(q, "USD")
	dashboard = NewDashboardService(events, notes, customTasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error from custom Overdue")
	}
	if !strings.Contains(err.Error(), "forced overdue error") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test transactions error
	db, err = database.Open(filepath.Join(t.TempDir(), "dashboard10d.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS transactions"); err != nil {
		t.Fatalf("drop transactions table: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q = generated.New(db)
	events = NewEventService(q)
	notes = NewNoteService(q)
	tasks = NewTaskService(q)
	transactions = NewTransactionService(q, "USD")
	dashboard = NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error from dropped transactions table")
	}

	// Test notes error
	db, err = database.Open(filepath.Join(t.TempDir(), "dashboard10e.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS notes"); err != nil {
		t.Fatalf("drop notes table: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q = generated.New(db)
	events = NewEventService(q)
	notes = NewNoteService(q)
	tasks = NewTaskService(q)
	transactions = NewTransactionService(q, "USD")
	dashboard = NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error from dropped notes table")
	}

	// Test balance error (uses transactions and wallets)
	db, err = database.Open(filepath.Join(t.TempDir(), "dashboard10f.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS wallets"); err != nil {
		t.Fatalf("drop wallets table: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q = generated.New(db)
	events = NewEventService(q)
	notes = NewNoteService(q)
	tasks = NewTaskService(q)
	transactions = NewTransactionService(q, "USD")
	dashboard = NewDashboardService(events, notes, tasks, transactions)

	_, err = dashboard.Get(context.Background(), "2026-10-29")
	if err == nil {
		t.Fatal("expected error from dropped wallets table")
	}
}
