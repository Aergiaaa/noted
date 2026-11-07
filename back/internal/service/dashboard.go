package service

import (
	"context"
	"time"
)

// TaskServicer defines the task operations needed by DashboardService.
// Exists as an interface to allow test mocking of the Overdue error path.
type TaskServicer interface {
	List(ctx context.Context, status, due *string) ([]Task, error)
	Overdue(ctx context.Context, day string) ([]Task, error)
}

type DashboardService struct {
	events       *EventService
	notes        *NoteService
	tasks        TaskServicer
	transactions *TransactionService
}

func NewDashboardService(
	events *EventService,
	notes *NoteService,
	tasks TaskServicer,
	transactions *TransactionService,
) *DashboardService {
	return &DashboardService{
		events:       events,
		notes:        notes,
		tasks:        tasks,
		transactions: transactions,
	}
}

// Dashboard is the composed read: one slice per card (ARCHITECTURE.md
// read-composition). It carries no JSON tags — the wire shape lives in
// handler/dashboard.go, like every other domain view.
type Dashboard struct {
	Date               string
	Balance            Balance
	EventsToday        []Event
	TasksToday         []Task
	OverdueTasks       []Task
	RecentTransactions []Transaction
	PinnedNotes        []Note
}

func (s *DashboardService) Get(ctx context.Context, day string) (Dashboard, error) {
	// Validate date format
	_, err := time.Parse("2006-01-02", day)
	if err != nil {
		return Dashboard{}, &ValidationError{Fields: map[string]string{
			"date": "must be YYYY-MM-DD",
		}}
	}

	// Get events today
	dayStart, _ := time.Parse("2006-01-02", day)
	dayEnd := dayStart.Add(24 * time.Hour)
	eventsToday, err := s.events.List(ctx, dayStart, dayEnd)
	if err != nil {
		return Dashboard{}, err
	}

	// Get tasks today
	openStatus := "open"
	tasksToday, err := s.tasks.List(ctx, &openStatus, &day)
	if err != nil {
		return Dashboard{}, err
	}

	// Get overdue tasks
	overdueTasks, err := s.tasks.Overdue(ctx, day)
	if err != nil {
		return Dashboard{}, err
	}

	// Get recent transactions (limit 5)
	txList, err := s.transactions.List(ctx, TransactionListParams{
		Limit: 5,
	})
	if err != nil {
		return Dashboard{}, err
	}

	// Get pinned notes (limit 5)
	pinnedTrue := true
	allNotes, err := s.notes.List(ctx, &pinnedTrue)
	if err != nil {
		return Dashboard{}, err
	}
	pinnedNotes := allNotes
	if len(pinnedNotes) > 5 {
		pinnedNotes = pinnedNotes[:5]
	}

	// Get balance
	balance, err := s.transactions.Balance(ctx)
	if err != nil {
		return Dashboard{}, err
	}

	return Dashboard{
		Date:               day,
		Balance:            balance,
		EventsToday:        eventsToday,
		TasksToday:         tasksToday,
		OverdueTasks:       overdueTasks,
		RecentTransactions: txList,
		PinnedNotes:        pinnedNotes,
	}, nil
}
