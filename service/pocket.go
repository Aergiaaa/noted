package service

import (
	"context"

	"github.com/Aergiaaa/noted/internal/database"
)

type PocketKind string

const (
	CASH    PocketKind = "cash"
	BANK    PocketKind = "bank"
	EWALLET PocketKind = "ewallet"
)

type PocketServicer interface {
	Create(ctx context.Context, name string, kind string) (database.CreatePocketRow, error)
	Delete(ctx context.Context, id string) error
	GetAll(ctx context.Context) ([]database.GetAllPocketsRow, error)
	Restore(ctx context.Context, id string) error
	Update(ctx context.Context, name, kind, id string) (database.UpdatePocketRow, error)
}

type PocketService struct {
	models *database.Queries
}

func (p *PocketService) Create(ctx context.Context, name, kind string) (database.CreatePocketRow, error) {
	if name == "" {
		return database.CreatePocketRow{}, ErrEmptyName
	}

	switch PocketKind(kind) {
	case CASH, BANK, EWALLET:
	default:
		return database.CreatePocketRow{}, ErrPocketKindMissmatch
	}

	params := database.CreatePocketParams{
		Name: name,
		Type: kind,
	}

	return p.models.CreatePocket(ctx, params)
}

func (p *PocketService) Delete(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return p.models.DeletePocket(ctx, cleanId)
}

func (p *PocketService) GetAll(ctx context.Context) ([]database.GetAllPocketsRow, error) {
	return p.models.GetAllPockets(ctx)
}

func (p *PocketService) Restore(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return p.models.RestorePocket(ctx, cleanId)
}

func (p *PocketService) Update(ctx context.Context, name, kind, id string) (database.UpdatePocketRow, error) {
	if name == "" {
		return database.UpdatePocketRow{}, ErrEmptyName
	}

	switch PocketKind(kind) {
	case CASH, BANK, EWALLET:
	default:
		return database.UpdatePocketRow{}, ErrPocketKindMissmatch
	}

	cleanId, err := parseUUID(id)
	if err != nil {
		return database.UpdatePocketRow{}, err
	}

	params := database.UpdatePocketParams{
		Name: name,
		Type: kind,
		ID:   cleanId,
	}

	return p.models.UpdatePocket(ctx, params)
}
