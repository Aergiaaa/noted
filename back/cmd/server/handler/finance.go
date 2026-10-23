package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// Page bounds for ?limit (API.md pagination).
const (
	TRANSACTION_PAGE_DEFAULT = 50
	TRANSACTION_PAGE_MAX     = 200
)

// walletRequest is the POST /api/finance/wallets body (API.md).
type walletRequest struct {
	Name string `json:"name"`
}

// walletPatchRequest: at least one field; name renames, archived archives.
type walletPatchRequest struct {
	Name     *string `json:"name"`
	Archived *bool   `json:"archived"`
}

type walletJSON struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ArchivedAt *string `json:"archived_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type walletsListResponse struct {
	Data []walletJSON `json:"data"`
}

// balanceResponse pins GET /api/finance/balance: one total plus the
// per-wallet split (API.md), all in the deployment currency.
type balanceResponse struct {
	Currency string              `json:"currency"`
	Total    balanceAmountJSON   `json:"total"`
	Wallets  []walletBalanceJSON `json:"wallets"`
}

type balanceAmountJSON struct {
	BalanceCents int64 `json:"balance_cents"`
}

type walletBalanceJSON struct {
	WalletID     string `json:"wallet_id"`
	Name         string `json:"name"`
	BalanceCents int64  `json:"balance_cents"`
}

// transactionRequest is the POST /api/finance/transactions body: income
// and expense only (kind=transfer answers 400 pointing at /transfers).
type transactionRequest struct {
	Kind        string  `json:"kind"`
	WalletID    string  `json:"wallet_id"`
	AmountCents int64   `json:"amount_cents"`
	OccurredOn  string  `json:"occurred_on"`
	Memo        *string `json:"memo"`
	TaskID      *string `json:"task_id"`
	EventID     *string `json:"event_id"`
	NoteID      *string `json:"note_id"`
}

// transferRequest is the POST /api/finance/transfers body.
type transferRequest struct {
	FromWalletID string  `json:"from_wallet_id"`
	ToWalletID   string  `json:"to_wallet_id"`
	AmountCents  int64   `json:"amount_cents"`
	OccurredOn   string  `json:"occurred_on"`
	Memo         *string `json:"memo"`
}

// transactionPatchRequest: kind is intentionally absent — it is immutable
// (void + recreate to change it), and DisallowUnknownFields makes a
// drifted client sending it fail loudly.
type transactionPatchRequest struct {
	AmountCents  *int64  `json:"amount_cents"`
	OccurredOn   *string `json:"occurred_on"`
	Memo         *string `json:"memo"`
	WalletID     *string `json:"wallet_id"`
	FromWalletID *string `json:"from_wallet_id"`
	ToWalletID   *string `json:"to_wallet_id"`
	TaskID       *string `json:"task_id"`
	EventID      *string `json:"event_id"`
	NoteID       *string `json:"note_id"`
}

// transactionJSON is the wire shape the frontend Transaction type pins.
type transactionJSON struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	AmountCents  int64   `json:"amount_cents"`
	Currency     string  `json:"currency"`
	OccurredOn   string  `json:"occurred_on"`
	Memo         *string `json:"memo"`
	WalletID     *string `json:"wallet_id"`
	FromWalletID *string `json:"from_wallet_id"`
	ToWalletID   *string `json:"to_wallet_id"`
	TaskID       *string `json:"task_id"`
	EventID      *string `json:"event_id"`
	NoteID       *string `json:"note_id"`
	CreatedAt    string  `json:"created_at"`
}

// transactionsListResponse pins the paged history envelope (API.md).
type transactionsListResponse struct {
	Data       []transactionJSON `json:"data"`
	NextCursor *string           `json:"next_cursor"`
}

func (h *Handler) ListWallets(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("archived") == "1"

	wallets, err := h.wallets.List(r.Context(), includeArchived)
	if err != nil {
		writeServiceError(w, err, "list wallets")
		return
	}

	data := make([]walletJSON, len(wallets))
	for i, wallet := range wallets {
		data[i] = toWalletJSON(wallet)
	}
	WriteJSON(w, http.StatusOK, walletsListResponse{Data: data})
}

func (h *Handler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	var req walletRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	wallet, err := h.wallets.Create(r.Context(), req.Name)
	if err != nil {
		writeServiceError(w, err, "create wallet")
		return
	}

	WriteJSON(w, http.StatusCreated, toWalletJSON(wallet))
}

func (h *Handler) UpdateWallet(w http.ResponseWriter, r *http.Request) {
	var req walletPatchRequest
	if !DecodeJSON(w, r, &req) {
		return
	}
	if req.Name == nil && req.Archived == nil {
		writeFieldError(w, "name", "name or archived required")
		return
	}

	id := chi.URLParam(r, "id")
	var wallet service.Wallet
	var err error
	if req.Name != nil {
		wallet, err = h.wallets.Rename(r.Context(), id, *req.Name)
		if err != nil {
			writeServiceError(w, err, "rename wallet")
			return
		}
	}
	if req.Archived != nil {
		wallet, err = h.wallets.Archive(r.Context(), id, *req.Archived)
		if err != nil {
			writeServiceError(w, err, "archive wallet")
			return
		}
	}

	WriteJSON(w, http.StatusOK, toWalletJSON(wallet))
}

func (h *Handler) FinanceBalance(w http.ResponseWriter, r *http.Request) {
	balance, err := h.transactions.Balance(r.Context())
	if err != nil {
		writeServiceError(w, err, "balance")
		return
	}

	wallets := make([]walletBalanceJSON, 0, len(balance.Wallets))
	for _, wallet := range balance.Wallets {
		wallets = append(wallets, walletBalanceJSON{
			WalletID:     wallet.WalletID,
			Name:         wallet.Name,
			BalanceCents: wallet.BalanceCents,
		})
	}
	WriteJSON(w, http.StatusOK, balanceResponse{
		Currency: balance.Currency,
		Total:    balanceAmountJSON{BalanceCents: balance.TotalCents},
		Wallets:  wallets,
	})
}

// ListTransactions: ?limit (1..200, default 50) + ?cursor (opaque) +
// ?wallet_id (matches either side of a transfer). next_cursor is null
// when the page did not fill.
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := int64(TRANSACTION_PAGE_DEFAULT)
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 || parsed > TRANSACTION_PAGE_MAX {
			writeFieldError(w, "limit", "must be between 1 and 200")
			return
		}
		limit = parsed
	}

	params := service.TransactionListParams{Limit: limit}
	if raw := q.Get("cursor"); raw != "" {
		occurredOn, id, ok := decodeTransactionCursor(raw)
		if !ok {
			writeFieldError(w, "cursor", "malformed")
			return
		}
		params.CursorOccurredOn = &occurredOn
		params.CursorID = &id
	}
	if walletID := q.Get("wallet_id"); walletID != "" {
		params.WalletID = &walletID
	}

	page, err := h.transactions.List(r.Context(), params)
	if err != nil {
		writeServiceError(w, err, "list transactions")
		return
	}

	var nextCursor *string
	if int64(len(page)) == limit && len(page) > 0 {
		last := page[len(page)-1]
		encoded := encodeTransactionCursor(last.OccurredOn, last.ID)
		nextCursor = &encoded
	}

	data := make([]transactionJSON, len(page))
	for i, row := range page {
		data[i] = toTransactionJSON(row)
	}
	WriteJSON(w, http.StatusOK, transactionsListResponse{Data: data, NextCursor: nextCursor})
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req transactionRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	walletID := req.WalletID
	transaction, err := h.transactions.Create(r.Context(), service.TransactionInput{
		Kind:        req.Kind,
		WalletID:    &walletID,
		AmountCents: req.AmountCents,
		OccurredOn:  req.OccurredOn,
		Memo:        req.Memo,
		TaskID:      req.TaskID,
		EventID:     req.EventID,
		NoteID:      req.NoteID,
	})
	if err != nil {
		writeServiceError(w, err, "create transaction")
		return
	}

	WriteJSON(w, http.StatusCreated, toTransactionJSON(transaction))
}

func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	from, to := req.FromWalletID, req.ToWalletID
	transaction, err := h.transactions.CreateTransfer(r.Context(), service.TransactionInput{
		FromWalletID: &from,
		ToWalletID:   &to,
		AmountCents:  req.AmountCents,
		OccurredOn:   req.OccurredOn,
		Memo:         req.Memo,
	})
	if err != nil {
		writeServiceError(w, err, "create transfer")
		return
	}

	WriteJSON(w, http.StatusCreated, toTransactionJSON(transaction))
}

func (h *Handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	var req transactionPatchRequest
	if !DecodeJSON(w, r, &req) {
		return
	}

	transaction, err := h.transactions.Update(r.Context(), chi.URLParam(r, "id"), service.TransactionPatch{
		AmountCents:  req.AmountCents,
		OccurredOn:   req.OccurredOn,
		Memo:         req.Memo,
		WalletID:     req.WalletID,
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		TaskID:       req.TaskID,
		EventID:      req.EventID,
		NoteID:       req.NoteID,
	})
	if err != nil {
		writeServiceError(w, err, "update transaction")
		return
	}

	WriteJSON(w, http.StatusOK, toTransactionJSON(transaction))
}

// DeleteTransaction voids: idempotent 204, soft-delete only (DOMAIN.md).
func (h *Handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	if err := h.transactions.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err, "delete transaction")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Cursor codec: base64url("YYYY-MM-DD|id") — opaque on the wire, the
// keyset pair underneath (API.md). Both parts validate on decode so a
// tampered cursor fails as a 400 field error, never a 500.
func encodeTransactionCursor(occurredOn, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(occurredOn + "|" + id))
}

func decodeTransactionCursor(raw string) (occurredOn, id string, ok bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", false
	}
	occurredOn, id, found := strings.Cut(string(decoded), "|")
	if !found || !validDate(occurredOn) || id == "" {
		return "", "", false
	}
	return occurredOn, id, true
}

func toWalletJSON(w service.Wallet) walletJSON {
	return walletJSON{
		ID:         w.ID,
		Name:       w.Name,
		ArchivedAt: w.ArchivedAt,
		CreatedAt:  w.CreatedAt,
		UpdatedAt:  w.UpdatedAt,
	}
}

func toTransactionJSON(t service.Transaction) transactionJSON {
	return transactionJSON{
		ID:           t.ID,
		Kind:         t.Kind,
		AmountCents:  t.AmountCents,
		Currency:     t.Currency,
		OccurredOn:   t.OccurredOn,
		Memo:         t.Memo,
		WalletID:     t.WalletID,
		FromWalletID: t.FromWalletID,
		ToWalletID:   t.ToWalletID,
		TaskID:       t.TaskID,
		EventID:      t.EventID,
		NoteID:       t.NoteID,
		CreatedAt:    t.CreatedAt,
	}
}
