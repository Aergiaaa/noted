package service

import (
	"context"
	"errors"

	"github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EdgeServicer interface {
	Create(ctx context.Context, arg CreateEdgeArg) error
	Delete(ctx context.Context, arg DeleteEdgeArg) error
	GetAll(ctx context.Context) ([]database.GetEdgesRow, error)
	SyncWikiLinks(ctx context.Context, pageId string, titles []string) error
}

type EdgeService struct {
	models *database.Queries
	pool   *pgxpool.Pool
}

type CreateEdgeArg struct {
	FromID   string
	FromType string
	ToID     string
	ToType   string
	LinkType string
}

type EdgeLinkType string

const (
	EDGE_WIKI_LINK    EdgeLinkType = "wiki-link"
	EDGE_PARENT_LINK  EdgeLinkType = "parent-link"
	EDGE_FINANCE_LINK EdgeLinkType = "finance-link"
)

func (e *EdgeService) Create(ctx context.Context, arg CreateEdgeArg) error {
	fromId, err := parseUUID(arg.FromID)
	if err != nil {
		return err
	}

	switch PagTrType(arg.FromType) {
	case PAGE, TRANSACTION:
	default:
		return ErrEdgeFromTypeMismatch
	}

	toId, err := parseUUID(arg.ToID)
	if err != nil {
		return err
	}

	switch PagTrType(arg.ToType) {
	case PAGE, TRANSACTION:
	default:
		return ErrEdgeToTypeMismatch
	}

	switch EdgeLinkType(arg.LinkType) {
	case EDGE_FINANCE_LINK, EDGE_PARENT_LINK, EDGE_WIKI_LINK:
	default:
		return ErrEdgeLinkTypeMismatch
	}

	params := database.CreateEdgeParams{
		FromID:   fromId,
		FromType: arg.FromType,
		ToID:     toId,
		ToType:   arg.ToType,
		LinkType: arg.LinkType,
	}
	return e.models.CreateEdge(ctx, params)
}

type DeleteEdgeArg struct {
	FromId   string
	ToId     string
	LinkType string
}

func (e *EdgeService) Delete(ctx context.Context, arg DeleteEdgeArg) error {
	fromId, err := parseUUID(arg.FromId)
	if err != nil {
		return err
	}

	toId, err := parseUUID(arg.ToId)
	if err != nil {
		return err
	}

	switch EdgeLinkType(arg.LinkType) {
	case EDGE_FINANCE_LINK, EDGE_PARENT_LINK, EDGE_WIKI_LINK:
	default:
		return ErrEdgeLinkTypeMismatch
	}

	params := database.DeleteEdgeParams{
		FromID:   fromId,
		ToID:     toId,
		LinkType: arg.LinkType,
	}

	return e.models.DeleteEdge(ctx, params)
}

func (e *EdgeService) GetAll(ctx context.Context) ([]database.GetEdgesRow, error) {
	return e.models.GetEdges(ctx)
}

func (e *EdgeService) SyncWikiLinks(ctx context.Context, pageId string, titles []string) error {
	pageUUID, err := parseUUID(pageId)
	if err != nil {
		return err
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := database.New(tx)

	err = q.DeleteEdgesFromPage(ctx, pageUUID)
	if err != nil {
		return err
	}

	for _, title := range titles {
		targetId, err := q.GetPageIdByTitle(ctx, title)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}

		err = q.CreateEdge(ctx, database.CreateEdgeParams{
			FromID:   pageUUID,
			FromType: string(PAGE),
			ToID:     targetId,
			ToType:   string(PAGE),
			LinkType: string(EDGE_WIKI_LINK),
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
