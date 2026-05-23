package service

import (
	"context"
	"time"

	"github.com/Aergiaaa/noted/internal/database"
)

type PageServicer interface {
	Create(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error)
	Delete(ctx context.Context, id string) error
	GetAll(ctx context.Context) ([]database.GetAllPagesRow, error)
	GetBackLinks(ctx context.Context, id string) ([]database.GetBacklinksRow, error)
	GetById(ctx context.Context, id string) (database.GetPageByIDRow, error)
	Restore(ctx context.Context, id string) error
	Update(ctx context.Context, title string, id string, date *time.Time) (database.UpdatePageRow, error)
}

type PageService struct {
	models *database.Queries
}

func (p *PageService) Create(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error) {
	if title == "" {
		return database.CreatePageRow{}, ErrEmptyTitle
	}

	cleanDate, err := parseDate(*date)
	if err != nil {
		return database.CreatePageRow{}, err
	}

	params := database.CreatePageParams{
		Title: title,
		Date:  cleanDate,
	}

	return p.models.CreatePage(ctx, params)
}

func (p *PageService) Delete(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return p.models.DeletePage(ctx, cleanId)
}

func (p *PageService) GetAll(ctx context.Context) ([]database.GetAllPagesRow, error) {
	return p.models.GetAllPages(ctx)
}

func (p *PageService) GetById(ctx context.Context, id string) (database.GetPageByIDRow, error) {
	cleanId, err := parseUUID(id)
	if err != nil {
		return database.GetPageByIDRow{}, err
	}

	return p.models.GetPageByID(ctx, cleanId)
}

func (p *PageService) Restore(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return p.models.RestorePage(ctx, cleanId)
}

func (p *PageService) Update(ctx context.Context, title, id string, date *time.Time) (database.UpdatePageRow, error) {
	if title == "" {
		return database.UpdatePageRow{}, ErrEmptyTitle
	}

	cleanId, err := parseUUID(id)
	if err != nil {
		return database.UpdatePageRow{}, err
	}

	cleanDate, err := parseDate(*date)
	if err != nil {
		return database.UpdatePageRow{}, err
	}

	params := database.UpdatePageParams{
		Title: title,
		ID:    cleanId,
		Date:  cleanDate,
	}

	return p.models.UpdatePage(ctx, params)
}

func (p *PageService) GetBackLinks(ctx context.Context, id string) ([]database.GetBacklinksRow, error) {
	cleanId, err := parseUUID(id)
	if err != nil {
		return []database.GetBacklinksRow{}, err
	}

	return p.models.GetBacklinks(ctx, cleanId)
}
