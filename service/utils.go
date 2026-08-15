package service

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrEmptyName                = errors.New("name cannot be empty")
	ErrEmptyColor               = errors.New("color cannot be empty")
	ErrPocketKindMissmatch      = errors.New("missmatch pocket kind")
	ErrBlockKindMissmatch       = errors.New("missmatch block kind")
	ErrTransactionTypeMissmatch = errors.New("missmatch transaction type")
	ErrEdgeFromTypeMissmatch    = errors.New("missmatch from edge type")
	ErrEdgeToTypeMissmatch      = errors.New("missmatch to edge type")
	ErrTagTargetTypeMissmatch   = errors.New("missmatch tag target type")
	ErrEdgeLinkTypeMissmatch    = errors.New("missmatch edge link type")
	ErrEmptyTitle               = errors.New("title cannot be empty")
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
