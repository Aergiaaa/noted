package service

import (
	"errors"
	"math"
	"strconv"
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
	ErrInvalidAmount           = errors.New("amount must be a finite number")
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

	var err error
	switch v := any(num).(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return pgtype.Numeric{}, ErrInvalidAmount
		}
		err = res.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	case int:
		err = res.Scan(strconv.FormatInt(int64(v), 10))
	case int32:
		err = res.Scan(strconv.FormatInt(int64(v), 10))
	default:
		err = res.Scan(num)
	}

	if err != nil {
		return pgtype.Numeric{}, err
	}

	return res, nil
}
