package service

import (
	"context"
	"time"

	"github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
)

type PageServicer interface {
	Create(ctx context.Context, title string, date *time.Time) (database.CreatePageRow, error)
	Delete(ctx context.Context, id string) error
	GetAll(ctx context.Context) ([]database.GetAllPagesRow, error)
	GetBackLinks(ctx context.Context, id string) ([]database.GetBacklinksRow, error)
	GetById(ctx context.Context, id string) (database.GetPageByIDRow, error)
	GetPagePaginated(ctx context.Context, page int, limit int) ([]database.GetPagePaginatedRow, error)
	GetTotalPage(ctx context.Context) (int32, error)
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

	var (
		cleanDate pgtype.Date
		err       error
	)
	if date != nil {
		cleanDate, err = parseDate(*date)
		if err != nil {
			return database.CreatePageRow{}, err
		}
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

	var (
		cleanDate pgtype.Date
		dateErr   error
	)
	if date != nil {
		cleanDate, dateErr = parseDate(*date)
		if dateErr != nil {
			return database.UpdatePageRow{}, dateErr
		}
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

func (p *PageService) GetPagePaginated(ctx context.Context, page, limit int) ([]database.GetPagePaginatedRow, error) {
	offset := (page - 1) * limit

	params := database.GetPagePaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	return p.models.GetPagePaginated(ctx, params)
}

func (p *PageService) GetTotalPage(ctx context.Context) (int32, error) {
	return p.models.CountPages(ctx)
}
