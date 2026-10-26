package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"noted/internal/database/generated"
)

// Kind names a transaction kind (DOMAIN.md); the DB CHECK mirrors the
// three values. A named type keeps an arbitrary string from landing in a
// Transaction without an explicit conversion at the boundary. kind is
// immutable on PATCH — converting is void + recreate (API.md).
type Kind string

const (
	KIND_INCOME   Kind = "income"
	KIND_EXPENSE  Kind = "expense"
	KIND_TRANSFER Kind = "transfer"
)

const (
	AMOUNT_MIN_CENTS = 1
	AMOUNT_MAX_CENTS = 1_000_000_000_000
	MEMO_MAX_CHAR    = 500
)

// TransactionInput is a fully-resolved transaction: a create payload, or
// a PATCH row after the service merged it over the stored one. Kind
// arrives forced by Create/CreateTransfer on the create path and is
// carried from the stored row on updates. Optional fields use "" for "no
// value" (no memo / no wallet side / no link), resolved to SQL NULL at
// the write boundary.
type TransactionInput struct {
	Kind         Kind
	AmountCents  int64
	OccurredOn   string
	Memo         string
	WalletID     string // income/expense only
	FromWalletID string // transfer only
	ToWalletID   string // transfer only
	TaskID       string
	EventID      string
	NoteID       string
}

// TransactionPatch: a nil field keeps the stored value; "" clears memo
// and the wallet/link fields (nulled at write). Kind is not
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
	Kind         Kind
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

// TransactionListParams: a nil Cursor means the first page; WalletID
// filters to either side of a transfer.
type TransactionListParams struct {
	WalletID *string
	Cursor   *TransactionCursor
	Limit    int64
}

// TransactionCursor is the opaque keyset pair (occurred_on, id) a page
// resumes from. Both parts travel together — never two loose pointers.
type TransactionCursor struct {
	OccurredOn string
	ID         string
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
	in.WalletID = ""
	return s.create(ctx, in)
}

func (s *TransactionService) create(ctx context.Context, in TransactionInput) (Transaction, error) {
	in.Memo = sanitizeLine(in.Memo)
	if err := s.validate(ctx, in, linkScope{task: true, event: true, note: true}); err != nil {
		return Transaction{}, err
	}

	row, err := s.q.CreateTransaction(ctx, generated.CreateTransactionParams{
		ID:           uuid.NewString(),
		Kind:         string(in.Kind),
		AmountCents:  in.AmountCents,
		Currency:     s.currency,
		OccurredOn:   in.OccurredOn,
		Memo:         nullIfEmpty(in.Memo),
		WalletID:     nullIfEmpty(in.WalletID),
		FromWalletID: nullIfEmpty(in.FromWalletID),
		ToWalletID:   nullIfEmpty(in.ToWalletID),
		TaskID:       nullIfEmpty(in.TaskID),
		EventID:      nullIfEmpty(in.EventID),
		NoteID:       nullIfEmpty(in.NoteID),
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
	var cursorOccurredOn, cursorID *string
	if p.Cursor != nil {
		cursorOccurredOn = &p.Cursor.OccurredOn
		cursorID = &p.Cursor.ID
	}

	rows, err := s.q.ListTransactionsPage(ctx, generated.ListTransactionsPageParams{
		CursorOccurredOn: cursorOccurredOn,
		CursorID:         cursorID,
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
	merged.Memo = sanitizeLine(merged.Memo)
	if err := s.validate(ctx, merged, linkScope{
		task:  changedLink(patch.TaskID, row.TaskID),
		event: changedLink(patch.EventID, row.EventID),
		note:  changedLink(patch.NoteID, row.NoteID),
	}); err != nil {
		return Transaction{}, err
	}

	updated, err := s.q.UpdateTransaction(ctx, generated.UpdateTransactionParams{
		Kind:         string(merged.Kind),
		AmountCents:  merged.AmountCents,
		Currency:     s.currency,
		OccurredOn:   merged.OccurredOn,
		Memo:         nullIfEmpty(merged.Memo),
		WalletID:     nullIfEmpty(merged.WalletID),
		FromWalletID: nullIfEmpty(merged.FromWalletID),
		ToWalletID:   nullIfEmpty(merged.ToWalletID),
		TaskID:       nullIfEmpty(merged.TaskID),
		EventID:      nullIfEmpty(merged.EventID),
		NoteID:       nullIfEmpty(merged.NoteID),
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

	_, purgeErr := s.Purge(ctx, time.Now())
	logPurge("transactions", purgeErr)

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
		fields["amount_cents"] = fmt.Sprintf("must be at least %d", AMOUNT_MIN_CENTS)
	case in.AmountCents > AMOUNT_MAX_CENTS:
		fields["amount_cents"] = fmt.Sprintf("must be at most %d", AMOUNT_MAX_CENTS)
	}

	if !validDay(in.OccurredOn) {
		fields["occurred_on"] = "must be YYYY-MM-DD"
	}

	if len([]rune(in.Memo)) > MEMO_MAX_CHAR {
		fields["memo"] = fmt.Sprintf("must be at most %d characters", MEMO_MAX_CHAR)
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
		if in.FromWalletID == "" {
			fields["from_wallet_id"] = "required"
		}

		switch {
		case in.ToWalletID == "":
			fields["to_wallet_id"] = "required"
		case in.FromWalletID != "" && in.FromWalletID == in.ToWalletID:
			fields["to_wallet_id"] = "must differ from from_wallet_id"
		}

		if in.WalletID != "" {
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

	if in.WalletID == "" {
		fields["wallet_id"] = "required"
	} else if err := s.validateWalletRef(ctx, fields, "wallet_id", in.WalletID); err != nil {
		return err
	}

	if in.FromWalletID != "" {
		fields["from_wallet_id"] = "only transfers use from_wallet_id"
	}
	if in.ToWalletID != "" {
		fields["to_wallet_id"] = "only transfers use to_wallet_id"
	}

	return nil
}

// validateWalletRef records "unknown wallet" for a non-empty id; a
// storage failure aborts validation with the raw error.
func (s *TransactionService) validateWalletRef(ctx context.Context, fields map[string]string, key, id string) error {
	if id == "" {
		return nil
	}
	_, err := s.q.GetWallet(ctx, id)
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
		if err := s.checkRef(ctx, in.TaskID, func(ctx context.Context, id string) error {
			_, err := s.q.GetTask(ctx, id)
			return err
		}, fields, "task_id", "unknown task"); err != nil {
			return err
		}
	}
	if scope.event {
		if err := s.checkRef(ctx, in.EventID, func(ctx context.Context, id string) error {
			_, err := s.q.GetEvent(ctx, id)
			return err
		}, fields, "event_id", "unknown event"); err != nil {
			return err
		}
	}
	if scope.note {
		if err := s.checkRef(ctx, in.NoteID, func(ctx context.Context, id string) error {
			_, err := s.q.GetNote(ctx, id)
			return err
		}, fields, "note_id", "unknown note"); err != nil {
			return err
		}
	}
	return nil
}

// checkRef: an empty id means "no link" and skips the existence probe.
func (s *TransactionService) checkRef(ctx context.Context, id string, get func(context.Context, string) error, fields map[string]string, key, message string) error {
	if id == "" {
		return nil
	}

	err := get(ctx, id)
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
		Kind:         Kind(row.Kind),
		AmountCents:  row.AmountCents,
		OccurredOn:   row.OccurredOn,
		Memo:         derefString(row.Memo),
		WalletID:     derefString(row.WalletID),
		FromWalletID: derefString(row.FromWalletID),
		ToWalletID:   derefString(row.ToWalletID),
		TaskID:       derefString(row.TaskID),
		EventID:      derefString(row.EventID),
		NoteID:       derefString(row.NoteID),
	}

	if patch.AmountCents != nil {
		in.AmountCents = *patch.AmountCents
	}
	if patch.OccurredOn != nil {
		in.OccurredOn = *patch.OccurredOn
	}
	if patch.Memo != nil {
		in.Memo = *patch.Memo
	}
	if patch.WalletID != nil {
		in.WalletID = *patch.WalletID
	}
	if patch.FromWalletID != nil {
		in.FromWalletID = *patch.FromWalletID
	}
	if patch.ToWalletID != nil {
		in.ToWalletID = *patch.ToWalletID
	}
	if patch.TaskID != nil {
		in.TaskID = *patch.TaskID
	}
	if patch.EventID != nil {
		in.EventID = *patch.EventID
	}
	if patch.NoteID != nil {
		in.NoteID = *patch.NoteID
	}
	return in
}

func validDay(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func fromTransactionRow(row generated.Transaction) Transaction {
	return Transaction{
		ID:           row.ID,
		Kind:         Kind(row.Kind),
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
