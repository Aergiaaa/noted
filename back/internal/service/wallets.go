package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"noted/internal/database/generated"
)

// WALLET_NAME_MAX_CHAR: name ceiling (API.md wallet names).
const WALLET_NAME_MAX_CHAR = 50

// WALLET_SEED_CASH_ID: the wallet 0005 seeds so pickers always have a
// target. A fixed id (not a ULID) keeps every environment identical.
const WALLET_SEED_CASH_ID = "wal_cash"

// Wallet is the domain view of a row; archived wallets keep history and
// balances but leave pickers and the active strip (DOMAIN.md).
type Wallet struct {
	ID         string
	Name       string
	ArchivedAt *string
	CreatedAt  string
	UpdatedAt  string
}

type WalletService struct {
	q *generated.Queries
}

func NewWalletService(q *generated.Queries) *WalletService {
	return &WalletService{q: q}
}

// Get: a missing id is ErrNotFound (the handler maps it to 404).
func (s *WalletService) Get(ctx context.Context, id string) (Wallet, error) {
	row, err := s.q.GetWallet(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{}, ErrNotFound
	}
	if err != nil {
		return Wallet{}, err
	}

	return fromWalletRow(row), nil
}

// List: active wallets by default, archived ones included on request.
func (s *WalletService) List(ctx context.Context, includeArchived bool) ([]Wallet, error) {
	var filter any
	if includeArchived {
		filter = int64(1)
	}

	rows, err := s.q.ListWallets(ctx, filter)
	if err != nil {
		return nil, err
	}

	wallets := make([]Wallet, len(rows))
	for i, row := range rows {
		wallets[i] = fromWalletRow(row)
	}

	return wallets, nil
}

func (s *WalletService) Create(ctx context.Context, name string) (Wallet, error) {
	name = sanitizeLine(name)
	if err := validateWalletName(name); err != nil {
		return Wallet{}, err
	}

	now := formatStored(time.Now())
	row, err := s.q.CreateWallet(ctx, generated.CreateWalletParams{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Wallet{}, err
	}

	return fromWalletRow(row), nil
}

// Rename: re-validate through the same name rules as Create.
func (s *WalletService) Rename(ctx context.Context, id, name string) (Wallet, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Wallet{}, err
	}

	name = sanitizeLine(name)
	if err := validateWalletName(name); err != nil {
		return Wallet{}, err
	}

	row, err := s.q.UpdateWallet(ctx, generated.UpdateWalletParams{
		Name:       name,
		ArchivedAt: current.ArchivedAt,
		UpdatedAt:  formatStored(time.Now()),
		ID:         id,
	})
	if err != nil {
		return Wallet{}, err
	}

	return fromWalletRow(row), nil
}

// Archive: true stamps archived_at (wallet leaves pickers/strip), false
// clears it. No delete path exists — history must keep resolving.
func (s *WalletService) Archive(ctx context.Context, id string, archived bool) (Wallet, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Wallet{}, err
	}

	var stamp *string
	if archived {
		now := formatStored(time.Now())
		stamp = &now
	}

	row, err := s.q.UpdateWallet(ctx, generated.UpdateWalletParams{
		Name:       current.Name,
		ArchivedAt: stamp,
		UpdatedAt:  formatStored(time.Now()),
		ID:         id,
	})
	if err != nil {
		return Wallet{}, err
	}

	return fromWalletRow(row), nil
}

func validateWalletName(name string) error {
	fields := map[string]string{}
	switch {
	case name == "":
		fields["name"] = "required"
	case len([]rune(name)) > WALLET_NAME_MAX_CHAR:
		fields["name"] = "must be at most 50 characters"
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	return nil
}

func fromWalletRow(row generated.Wallet) Wallet {
	return Wallet{
		ID:         row.ID,
		Name:       row.Name,
		ArchivedAt: row.ArchivedAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
