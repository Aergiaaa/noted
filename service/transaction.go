package service

import (
	"context"
	"time"

	"github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransactionServicer interface {
	Create(ctx context.Context, arg CreateTransactionArg) (database.CreateTransactionRow, error)
	Delete(ctx context.Context, id string) error
	GetAll(ctx context.Context) ([]database.GetAllTransactionsRow, error)
	GetById(ctx context.Context, id string) (database.GetTransactionByIDRow, error)
	GetWithPockets(ctx context.Context) ([]database.GetTransactionsWithPocketNamesRow, error)
	Restore(ctx context.Context, id string) error
	Update(ctx context.Context, arg UpdateTransactionArg) (database.UpdateTransactionRow, error)
}

type TransactionService struct {
	models *database.Queries
}

type TransactionType string

const (
	TRANSACTION_INCOME   TransactionType = "income"
	TRANSACTION_EXPENSE  TransactionType = "expense"
	TRANSACTION_TRANSFER TransactionType = "transfer"
)

type CreateTransactionArg struct {
	Title        string
	Type         string
	Amount       float64
	Date         time.Time
	FromPocketID string
	ToPocketID   string
}

func (t *TransactionService) Create(ctx context.Context, arg CreateTransactionArg) (database.CreateTransactionRow, error) {
	if arg.Title == "" {
		return database.CreateTransactionRow{}, ErrEmptyTitle
	}

	trType := TransactionType(arg.Type)
	switch trType {
	case TRANSACTION_INCOME, TRANSACTION_EXPENSE, TRANSACTION_TRANSFER:
	default:
		return database.CreateTransactionRow{}, ErrTransactionTypeMismatch
	}

	cleanDate, err := parseDate(arg.Date)
	if err != nil {
		return database.CreateTransactionRow{}, err
	}

	cleanAmount, err := parseNum(arg.Amount)
	if err != nil {
		return database.CreateTransactionRow{}, err
	}

	var fromId pgtype.UUID
	if trType == TRANSACTION_EXPENSE || trType == TRANSACTION_TRANSFER {
		if arg.FromPocketID != "" {
			fromId, err = parseUUID(arg.FromPocketID)
			if err != nil {
				return database.CreateTransactionRow{}, err
			}
		}
	}

	var toId pgtype.UUID
	if trType == TRANSACTION_INCOME || trType == TRANSACTION_TRANSFER {
		if arg.ToPocketID != "" {
			toId, err = parseUUID(arg.ToPocketID)
			if err != nil {
				return database.CreateTransactionRow{}, err
			}
		}
	}

	params := database.CreateTransactionParams{
		Title:        arg.Title,
		Type:         arg.Type,
		Amount:       cleanAmount,
		Date:         cleanDate,
		FromPocketID: fromId,
		ToPocketID:   toId,
	}

	return t.models.CreateTransaction(ctx, params)
}

func (t *TransactionService) Delete(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return t.models.DeleteTransaction(ctx, cleanId)
}

func (t *TransactionService) GetAll(ctx context.Context) ([]database.GetAllTransactionsRow, error) {
	return t.models.GetAllTransactions(ctx)
}

func (t *TransactionService) GetById(ctx context.Context, id string) (database.GetTransactionByIDRow, error) {
	cleanId, err := parseUUID(id)
	if err != nil {
		return database.GetTransactionByIDRow{}, err
	}

	return t.models.GetTransactionByID(ctx, cleanId)
}

func (t *TransactionService) GetWithPockets(ctx context.Context) ([]database.GetTransactionsWithPocketNamesRow, error) {
	return t.models.GetTransactionsWithPocketNames(ctx)
}

func (t *TransactionService) Restore(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return t.models.RestoreTransaction(ctx, cleanId)
}

type UpdateTransactionArg struct {
	Title        string
	Type         string
	Amount       float64
	Date         time.Time
	FromPocketID string
	ToPocketID   string
	ID           string
}

func (t *TransactionService) Update(ctx context.Context, arg UpdateTransactionArg) (database.UpdateTransactionRow, error) {
	if arg.Title == "" {
		return database.UpdateTransactionRow{}, ErrEmptyTitle
	}

	trType := TransactionType(arg.Type)
	switch trType {
	case TRANSACTION_INCOME, TRANSACTION_EXPENSE, TRANSACTION_TRANSFER:
	default:
		return database.UpdateTransactionRow{}, ErrTransactionTypeMismatch
	}

	cleanDate, err := parseDate(arg.Date)
	if err != nil {
		return database.UpdateTransactionRow{}, err
	}

	cleanAmount, err := parseNum(arg.Amount)
	if err != nil {
		return database.UpdateTransactionRow{}, err
	}

	var fromId pgtype.UUID
	if trType == TRANSACTION_EXPENSE || trType == TRANSACTION_TRANSFER {
		if arg.FromPocketID != "" {
			fromId, err = parseUUID(arg.FromPocketID)
			if err != nil {
				return database.UpdateTransactionRow{}, err
			}
		}
	}

	var toId pgtype.UUID
	if trType == TRANSACTION_INCOME || trType == TRANSACTION_TRANSFER {
		if arg.ToPocketID != "" {
			toId, err = parseUUID(arg.ToPocketID)
			if err != nil {
				return database.UpdateTransactionRow{}, err
			}
		}
	}

	cleanId, err := parseUUID(arg.ID)
	if err != nil {
		return database.UpdateTransactionRow{}, err
	}

	params := database.UpdateTransactionParams{
		Title:        arg.Title,
		Type:         arg.Type,
		Amount:       cleanAmount,
		Date:         cleanDate,
		FromPocketID: fromId,
		ToPocketID:   toId,
		ID:           cleanId,
	}

	return t.models.UpdateTransaction(ctx, params)
}
