package service

import (
	"context"

	"github.com/Aergiaaa/noted/internal/database"
)

type TaggableServicer interface {
	Attach(ctx context.Context, arg AttachTagArg) error
	Detach(ctx context.Context, arg DetatchTagArg) error
	GetTagsByTargetId(ctx context.Context, id string, kind string) ([]database.GetTagsByTargetRow, error)
}

type TaggableService struct {
	models *database.Queries
}

type AttachTagArg struct {
	TagID      string
	TargetID   string
	TargetType string
}

func (t *TaggableService) Attach(ctx context.Context, arg AttachTagArg) error {
	tagId, err := parseUUID(arg.TagID)
	if err != nil {
		return err
	}

	targetId, err := parseUUID(arg.TargetID)
	if err != nil {
		return err
	}

	switch PagTrType(arg.TargetType) {
	case PAGE, TRANSACTION:
	default:
		return ErrTagTargetTypeMissmatch
	}

	params := database.AttachTagParams{
		TagID:      tagId,
		TargetID:   targetId,
		TargetType: arg.TargetType,
	}

	return t.models.AttachTag(ctx, params)
}

type DetatchTagArg struct {
	TagID      string
	TargetID   string
	TargetType string
}

func (t *TaggableService) Detach(ctx context.Context, arg DetatchTagArg) error {
	tagId, err := parseUUID(arg.TagID)
	if err != nil {
		return err
	}

	targetId, err := parseUUID(arg.TargetID)
	if err != nil {
		return err
	}

	switch PagTrType(arg.TargetType) {
	case PAGE, TRANSACTION:
	default:
		return ErrTagTargetTypeMissmatch
	}

	params := database.DetachTagParams{
		TagID:      tagId,
		TargetID:   targetId,
		TargetType: arg.TargetType,
	}

	return t.models.DetachTag(ctx, params)
}

func (t *TaggableService) GetTagsByTargetId(ctx context.Context, id, kind string) ([]database.GetTagsByTargetRow, error) {
	cleanId, err := parseUUID(id)
	if err != nil {
		return []database.GetTagsByTargetRow{}, err
	}

	switch PagTrType(kind) {
	case PAGE, TRANSACTION:
	default:
		return []database.GetTagsByTargetRow{}, ErrTagTargetTypeMissmatch
	}

	params := database.GetTagsByTargetParams{
		TargetID:   cleanId,
		TargetType: kind,
	}

	return t.models.GetTagsByTarget(ctx, params)
}
