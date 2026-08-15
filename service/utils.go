package service

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrEmptyName               = errors.New("name cannot be empty")
	ErrEmptyColor              = errors.New("color cannot be empty")
	ErrPocketKindMismatch      = errors.New("mismatch pocket kind")
	ErrBlockKindMismatch       = errors.New("mismatch block kind")
	ErrTransactionTypeMismatch = errors.New("mismatch transaction type")
	ErrEdgeFromTypeMismatch    = errors.New("mismatch from edge type")
	ErrEdgeToTypeMismatch      = errors.New("mismatch to edge type")
	ErrTagTargetTypeMismatch   = errors.New("mismatch tag target type")
	ErrEdgeLinkTypeMismatch    = errors.New("mismatch edge link type")
	ErrEmptyTitle              = errors.New("title cannot be empty")
)

type PagTrType string

const (
	PAGE        PagTrType = "page"
	TRANSACTION PagTrType = "transaction"
)

func parseUUID(id string) (pgtype.UUID, error) {
	var cleanId pgtype.UUID
	if err := cleanId.Scan(id); err != nil {
		return pgtype.UUID{}, err
	}

	return cleanId, nil
}

func parseDate(date time.Time) (pgtype.Date, error) {
	var cleanDate pgtype.Date
	if err := cleanDate.Scan(date); err != nil {
		return pgtype.Date{}, err
	}

	return cleanDate, nil
}

type Number interface {
	~int32 | ~float64 | ~int
}

func parseNum[T Number](num T) (pgtype.Numeric, error) {
	var res pgtype.Numeric

	if err := res.Scan(num); err != nil {
		return pgtype.Numeric{}, err
	}

	return res, nil
}
