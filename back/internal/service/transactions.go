package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"noted/internal/database/generated"
)

// Transaction kinds (DOMAIN.md); the DB CHECK mirrors them. kind is
// immutable on PATCH — converting is void + recreate (API.md).
const (
	KIND_INCOME   = "income"
	KIND_EXPENSE  = "expense"
	KIND_TRANSFER = "transfer"
)

const (
	AMOUNT_MIN_CENTS = 1
	AMOUNT_MAX_CENTS = 1e12
	MEMO_MAX_CHAR    = 500
)

// TransactionInput is a fully-resolved transaction: a create payload, or
// a PATCH row after the service merged it over the stored one. Kind
// arrives forced by Create/CreateTransfer on the create path and is
// carried from the stored row on updates.
type TransactionInput struct {
	Kind         string
	AmountCents  int64
	OccurredOn   string
	Memo         *string
	WalletID     *string // income/expense only
	FromWalletID *string // transfer only
	ToWalletID   *string // transfer only
	TaskID       *string
	EventID      *string
	NoteID       *string
}

// TransactionPatch: a nil field keeps the stored value; "" clears memo
// and the wallet/link fields (normalizeID at write). Kind is not
// patchable by design.
type TransactionPatch struct {
	AmountCents  *int64
	OccurredOn   *string
	Memo         *string
	WalletID     *string
	FromWalletID *string
	ToWalletID   *string
	TaskID       *string
	EventID      *string
	NoteID       *string
}

// Transaction is the domain view of a row; timestamps stay the stored
// UTC strings (already the wire format, like Event/Note).
type Transaction struct {
	ID           string
	Kind         string
	AmountCents  int64
	Currency     string
	OccurredOn   string
	Memo         *string
	WalletID     *string
	FromWalletID *string
	ToWalletID   *string
	TaskID       *string
	EventID      *string
	NoteID       *string
	CreatedAt    string
}

// TransactionListParams: nil cursor fields mean the first page.
type TransactionListParams struct {
	WalletID         *string
	CursorOccurredOn *string
	CursorID         *string
	Limit            int64
}

// Balance is the composed read: total over every live row plus one entry
// per active wallet (transfers split per-wallet, cancel in the total).
type Balance struct {
	Currency   string
	TotalCents int64
	Wallets    []WalletBalance
}

type WalletBalance struct {
	WalletID     string
	Name         string
	BalanceCents int64
}

type TransactionService struct {
	q        *generated.Queries
	currency string
}

// currency is the deployment constant (CURRENCY_DEFAULT) stamped on
// every write — never taken from the request (API.md).
func NewTransactionService(q *generated.Queries, currency string) *TransactionService {
	return &TransactionService{q: q, currency: currency}
}

func (s *TransactionService) Create(ctx context.Context, in TransactionInput) (Transaction, error) {
	if in.Kind != KIND_INCOME && in.Kind != KIND_EXPENSE {
		return Transaction{}, &ValidationError{Fields: map[string]string{
			"kind": "must be income or expense (use /transfers)",
		}}
	}
	return s.create(ctx, in)
}

func (s *TransactionService) CreateTransfer(ctx context.Context, in TransactionInput) (Transaction, error) {
	in.Kind = KIND_TRANSFER
	in.WalletID = nil
	return s.create(ctx, in)
}

func (s *TransactionService) create(ctx context.Context, in TransactionInput) (Transaction, error) {
	in.Memo = normalizeMemo(in.Memo)
	if err := s.validate(ctx, in, linkScope{task: true, event: true, note: true}); err != nil {
		return Transaction{}, err
	}

	row, err := s.q.CreateTransaction(ctx, generated.CreateTransactionParams{
		ID:           uuid.NewString(),
		Kind:         in.Kind,
		AmountCents:  in.AmountCents,
		Currency:     s.currency,
		OccurredOn:   in.OccurredOn,
		Memo:         in.Memo,
		WalletID:     normalizeID(in.WalletID),
		FromWalletID: normalizeID(in.FromWalletID),
		ToWalletID:   normalizeID(in.ToWalletID),
		TaskID:       normalizeID(in.TaskID),
		EventID:      normalizeID(in.EventID),
		NoteID:       normalizeID(in.NoteID),
		CreatedAt:    formatStored(time.Now()),
	})
	if err != nil {
		return Transaction{}, err
	}

	return fromTransactionRow(row), nil
}

func (s *TransactionService) Get(ctx context.Context, id string) (Transaction, error) {
	row, err := s.q.GetTransaction(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, err
	}

	return fromTransactionRow(row), nil
}

// List: keyset page, newest first; a nil wallet filter shows everything.
func (s *TransactionService) List(ctx context.Context, p TransactionListParams) ([]Transaction, error) {
	rows, err := s.q.ListTransactionsPage(ctx, generated.ListTransactionsPageParams{
		CursorOccurredOn: p.CursorOccurredOn,
		CursorID:         p.CursorID,
		WalletID:         p.WalletID,
		PageLimit:        p.Limit,
	})
	if err != nil {
		return nil, err
	}

	transactions := make([]Transaction, len(rows))
	for i, row := range rows {
		transactions[i] = fromTransactionRow(row)
	}

	return transactions, nil
}

// Balance composes per-wallet sums on read (ARCHITECTURE D4: nothing
// stored, nothing to drift). Archived wallets are excluded — they keep
// their rows but leave the strip.
func (s *TransactionService) Balance(ctx context.Context) (Balance, error) {
	totals, err := s.q.BalanceSums(ctx)
	if err != nil {
		return Balance{}, err
	}
	sums, err := s.q.BalanceSumsByWallet(ctx)
	if err != nil {
		return Balance{}, err
	}
	transfersIn, err := s.q.TransferInSumsByWallet(ctx)
	if err != nil {
		return Balance{}, err
	}
	transfersOut, err := s.q.TransferOutSumsByWallet(ctx)
	if err != nil {
		return Balance{}, err
	}
	wallets, err := s.q.ListWallets(ctx, nil)
	if err != nil {
		return Balance{}, err
	}

	perWallet := map[string]int64{}
	for _, sum := range sums {
		if sum.WalletID != nil {
			perWallet[*sum.WalletID] += sum.IncomeCents - sum.ExpenseCents
		}
	}
	for _, in := range transfersIn {
		if in.WalletID != nil {
			perWallet[*in.WalletID] += in.AmountCents
		}
	}
	for _, out := range transfersOut {
		if out.WalletID != nil {
			perWallet[*out.WalletID] -= out.AmountCents
		}
	}

	balance := Balance{
		Currency:   s.currency,
		TotalCents: totals.IncomeCents - totals.ExpenseCents,
		Wallets:    make([]WalletBalance, 0, len(wallets)),
	}
	for _, wallet := range wallets {
		balance.Wallets = append(balance.Wallets, WalletBalance{
			WalletID:     wallet.ID,
			Name:         wallet.Name,
			BalanceCents: perWallet[wallet.ID],
		})
	}

	return balance, nil
}

// Update reads the row, merges the patch over it (Kind stays stored),
// re-validates the whole, and keeps created_at (Update wants every
// column). Currency is re-stamped from config — single-currency app.
func (s *TransactionService) Update(ctx context.Context, id string, patch TransactionPatch) (Transaction, error) {
	row, err := s.q.GetTransaction(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, err
	}

	merged := mergeTransactionPatch(row, patch)
	merged.Memo = normalizeMemo(merged.Memo)
	if err := s.validate(ctx, merged, linkScope{
		task:  changedLink(patch.TaskID, row.TaskID),
		event: changedLink(patch.EventID, row.EventID),
		note:  changedLink(patch.NoteID, row.NoteID),
	}); err != nil {
		return Transaction{}, err
	}

	updated, err := s.q.UpdateTransaction(ctx, generated.UpdateTransactionParams{
		Kind:         merged.Kind,
		AmountCents:  merged.AmountCents,
		Currency:     s.currency,
		OccurredOn:   merged.OccurredOn,
		Memo:         merged.Memo,
		WalletID:     normalizeID(merged.WalletID),
		FromWalletID: normalizeID(merged.FromWalletID),
		ToWalletID:   normalizeID(merged.ToWalletID),
		TaskID:       normalizeID(merged.TaskID),
		EventID:      normalizeID(merged.EventID),
		NoteID:       normalizeID(merged.NoteID),
		CreatedAt:    row.CreatedAt,
		ID:           id,
	})
	if err != nil {
		return Transaction{}, err
	}

	return fromTransactionRow(updated), nil
}

// Delete voids: stamps deleted_at (204) and opportunistically purges rows
// past retention (ON DELETE SET NULL at purge, peers survive). Voided rows
// leave every balance. A purge failure never fails the void; boot retries —
// same tolerance as the event/note/task deletes.
func (s *TransactionService) Delete(ctx context.Context, id string) error {
	now := formatStored(time.Now())
	if err := s.q.SoftDeleteTransaction(ctx, generated.SoftDeleteTransactionParams{
		DeletedAt: &now,
		ID:        id,
	}); err != nil {
		return err
	}

	s.Purge(ctx, time.Now())

	return nil
}

func (s *TransactionService) Purge(ctx context.Context, now time.Time) (int64, error) {
	cutoff := formatStored(now.Add(-SOFT_DELETE_RETENTION))
	return s.q.PurgeTransactionsDeletedBefore(ctx, &cutoff)
}

func (s *TransactionService) validate(ctx context.Context, in TransactionInput, scope linkScope) error {
	fields := map[string]string{}

	switch {
	case in.AmountCents < AMOUNT_MIN_CENTS:
		fields["amount_cents"] = "must be at least 1"
	case in.AmountCents > AMOUNT_MAX_CENTS:
		fields["amount_cents"] = "must be at most 1000000000000"
	}

	if !validDay(in.OccurredOn) {
		fields["occurred_on"] = "must be YYYY-MM-DD"
	}

	if len([]rune(derefMemo(in.Memo))) > MEMO_MAX_CHAR {
		fields["memo"] = "must be at most 500 characters"
	}

	if err := s.validateWallets(ctx, in, fields); err != nil {
		return err
	}
	if err := s.validateLinks(ctx, in, fields, scope); err != nil {
		return err
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	return nil
}

// validateWallets enforces the per-kind exclusivity the schema cannot
// express cheaply (DOMAIN.md): income/expense need wallet_id and no
// transfer sides; transfers need distinct from/to and no wallet_id.
func (s *TransactionService) validateWallets(ctx context.Context, in TransactionInput, fields map[string]string) error {
	if in.Kind == KIND_TRANSFER {
		if normalizeID(in.FromWalletID) == nil {
			fields["from_wallet_id"] = "required"
		}

		from := derefID(in.FromWalletID)
		to := normalizeID(in.ToWalletID)
		switch {
		case to == nil:
			fields["to_wallet_id"] = "required"
		case from != "" && from == *to:
			fields["to_wallet_id"] = "must differ from from_wallet_id"
		}

		if normalizeID(in.WalletID) != nil {
			fields["wallet_id"] = "not used by transfers"
		}

		if err := errors.Join(
			s.validateWalletRef(ctx, fields, "from_wallet_id", in.FromWalletID),
			s.validateWalletRef(ctx, fields, "to_wallet_id", in.ToWalletID),
		); err != nil {
			return err
		}
		return nil
	}

	if normalizeID(in.WalletID) == nil {
		fields["wallet_id"] = "required"
	} else if err := s.validateWalletRef(ctx, fields, "wallet_id", in.WalletID); err != nil {
		return err
	}

	if normalizeID(in.FromWalletID) != nil {
		fields["from_wallet_id"] = "only transfers use from_wallet_id"
	}
	if normalizeID(in.ToWalletID) != nil {
		fields["to_wallet_id"] = "only transfers use to_wallet_id"
	}

	return nil
}

func (s *TransactionService) validateWalletRef(ctx context.Context, fields map[string]string, key string, id *string) error {
	if normalizeID(id) == nil {
		return nil
	}
	_, err := s.q.GetWallet(ctx, *normalizeID(id))
	if errors.Is(err, sql.ErrNoRows) {
		fields[key] = "unknown wallet"
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

func (s *TransactionService) validateLinks(ctx context.Context, in TransactionInput, fields map[string]string, scope linkScope) error {
	if scope.task {
		if err := s.checkRef(ctx, normalizeID(in.TaskID), func(ctx context.Context, id string) error {
			_, err := s.q.GetTask(ctx, id)
			return err
		}, fields, "task_id", "unknown task"); err != nil {
			return err
		}
	}
	if scope.event {
		if err := s.checkRef(ctx, normalizeID(in.EventID), func(ctx context.Context, id string) error {
			_, err := s.q.GetEvent(ctx, id)
			return err
		}, fields, "event_id", "unknown event"); err != nil {
			return err
		}
	}
	if scope.note {
		if err := s.checkRef(ctx, normalizeID(in.NoteID), func(ctx context.Context, id string) error {
			_, err := s.q.GetNote(ctx, id)
			return err
		}, fields, "note_id", "unknown note"); err != nil {
			return err
		}
	}
	return nil
}

func (s *TransactionService) checkRef(ctx context.Context, id *string, get func(context.Context, string) error, fields map[string]string, key, message string) error {
	if id == nil {
		return nil
	}

	err := get(ctx, *id)
	if errors.Is(err, sql.ErrNoRows) {
		fields[key] = message
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

func mergeTransactionPatch(row generated.Transaction, patch TransactionPatch) TransactionInput {
	in := TransactionInput{
		Kind:         row.Kind,
		AmountCents:  row.AmountCents,
		OccurredOn:   row.OccurredOn,
		Memo:         row.Memo,
		WalletID:     row.WalletID,
		FromWalletID: row.FromWalletID,
		ToWalletID:   row.ToWalletID,
		TaskID:       row.TaskID,
		EventID:      row.EventID,
		NoteID:       row.NoteID,
	}

	if patch.AmountCents != nil {
		in.AmountCents = *patch.AmountCents
	}
	if patch.OccurredOn != nil {
		in.OccurredOn = *patch.OccurredOn
	}
	if patch.Memo != nil {
		in.Memo = patch.Memo
	}
	if patch.WalletID != nil {
		in.WalletID = patch.WalletID
	}
	if patch.FromWalletID != nil {
		in.FromWalletID = patch.FromWalletID
	}
	if patch.ToWalletID != nil {
		in.ToWalletID = patch.ToWalletID
	}
	if patch.TaskID != nil {
		in.TaskID = patch.TaskID
	}
	if patch.EventID != nil {
		in.EventID = patch.EventID
	}
	if patch.NoteID != nil {
		in.NoteID = patch.NoteID
	}
	return in
}

// normalizeMemo: trim/sanitize and collapse "" to nil (memo is nullable).
func normalizeMemo(memo *string) *string {
	if memo == nil {
		return nil
	}
	clean := sanitizeLine(*memo)
	if clean == "" {
		return nil
	}
	return &clean
}

func derefMemo(memo *string) string {
	if memo == nil {
		return ""
	}
	return *memo
}

func derefID(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

func validDay(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func fromTransactionRow(row generated.Transaction) Transaction {
	return Transaction{
		ID:           row.ID,
		Kind:         row.Kind,
		AmountCents:  row.AmountCents,
		Currency:     row.Currency,
		OccurredOn:   row.OccurredOn,
		Memo:         row.Memo,
		WalletID:     row.WalletID,
		FromWalletID: row.FromWalletID,
		ToWalletID:   row.ToWalletID,
		TaskID:       row.TaskID,
		EventID:      row.EventID,
		NoteID:       row.NoteID,
		CreatedAt:    row.CreatedAt,
	}
}
