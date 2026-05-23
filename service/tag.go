package service

import (
	"context"

	"github.com/Aergiaaa/noted/internal/database"
)

type TagServicer interface {
	Create(ctx context.Context, name string, color string) (database.CreateTagRow, error)
	Delete(ctx context.Context, id string) error
	GetAll(ctx context.Context) ([]database.GetAllTagsRow, error)
	Restore(ctx context.Context, id string) error
	Update(ctx context.Context, name string, color string, id string) (database.UpdateTagRow, error)
}

type TagService struct {
	models *database.Queries
}

func (t *TagService) Create(ctx context.Context, name, color string) (database.CreateTagRow, error) {
	if name == "" {
		return database.CreateTagRow{}, ErrEmptyName
	}

	if color == "" {
		return database.CreateTagRow{}, ErrEmptyColor
	}

	params := database.CreateTagParams{
		Name:  name,
		Color: color,
	}

	return t.models.CreateTag(ctx, params)
}

func (t *TagService) Delete(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return t.models.DeleteTag(ctx, cleanId)
}

func (t *TagService) GetAll(ctx context.Context) ([]database.GetAllTagsRow, error) {
	return t.models.GetAllTags(ctx)
}

func (t *TagService) Restore(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return t.models.RestoreTag(ctx, cleanId)
}

func (t *TagService) Update(ctx context.Context, name, color, id string) (database.UpdateTagRow, error) {
	if name == "" {
		return database.UpdateTagRow{}, ErrEmptyName
	}

	if color == "" {
		return database.UpdateTagRow{}, ErrEmptyColor
	}

	cleanId, err := parseUUID(id)
	if err != nil {
		return database.UpdateTagRow{}, err
	}

	params := database.UpdateTagParams{
		Name:  name,
		Color: color,
		ID:    cleanId,
	}

	return t.models.UpdateTag(ctx, params)
}
