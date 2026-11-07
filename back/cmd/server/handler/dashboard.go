package handler

import (
	"net/http"
	"time"

	"noted/internal/service"
)

// dashboardResponse pins GET /api/dashboard (API.md) to the shape
// DashboardPage reads. The service structs are the domain view and carry
// no JSON tags, so the handler maps them onto the same per-row JSON types
// the module endpoints already use — marshalling service.Dashboard
// directly would answer PascalCase (StartsAt) and leave the page reading
// undefined.
type dashboardResponse struct {
	Date               string            `json:"date"`
	Balance            balanceResponse   `json:"balance"`
	EventsToday        []eventJSON       `json:"events_today"`
	TasksToday         []taskJSON        `json:"tasks_today"`
	OverdueTasks       []taskJSON        `json:"overdue_tasks"`
	RecentTransactions []transactionJSON `json:"recent_transactions"`
	PinnedNotes        []noteJSON        `json:"pinned_notes"`
}

func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	// Get date from query param, default to today in server TZ
	day := r.URL.Query().Get("date")
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}

	dashboard, err := h.dashboard.Get(r.Context(), day)
	if err != nil {
		writeServiceError(w, err, "dashboard")
		return
	}

	WriteJSON(w, http.StatusOK, toDashboardResponse(dashboard))
}

// toDashboardResponse: nil lists become [] so an empty card is an array
// (a null would crash the page's .filter()).
func toDashboardResponse(d service.Dashboard) dashboardResponse {
	events := make([]eventJSON, 0, len(d.EventsToday))
	for _, e := range d.EventsToday {
		events = append(events, toEventJSON(e))
	}

	tasksToday := make([]taskJSON, 0, len(d.TasksToday))
	for _, t := range d.TasksToday {
		tasksToday = append(tasksToday, toTaskJSON(t))
	}

	overdue := make([]taskJSON, 0, len(d.OverdueTasks))
	for _, t := range d.OverdueTasks {
		overdue = append(overdue, toTaskJSON(t))
	}

	transactions := make([]transactionJSON, 0, len(d.RecentTransactions))
	for _, tx := range d.RecentTransactions {
		transactions = append(transactions, toTransactionJSON(tx))
	}

	notes := make([]noteJSON, 0, len(d.PinnedNotes))
	for _, n := range d.PinnedNotes {
		notes = append(notes, toNoteJSON(n))
	}

	wallets := make([]walletBalanceJSON, 0, len(d.Balance.Wallets))
	for _, wallet := range d.Balance.Wallets {
		wallets = append(wallets, walletBalanceJSON{
			WalletID:     wallet.WalletID,
			Name:         wallet.Name,
			BalanceCents: wallet.BalanceCents,
		})
	}

	return dashboardResponse{
		Date: d.Date,
		Balance: balanceResponse{
			Currency: d.Balance.Currency,
			Total:    balanceAmountJSON{BalanceCents: d.Balance.TotalCents},
			Wallets:  wallets,
		},
		EventsToday:        events,
		TasksToday:         tasksToday,
		OverdueTasks:       overdue,
		RecentTransactions: transactions,
		PinnedNotes:        notes,
	}
}
