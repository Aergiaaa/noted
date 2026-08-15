package service

import (
	"context"

	"github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BlockType string

const (
	BLOCK_TEXT    BlockType = "text"
	BLOCK_HEADING BlockType = "heading"
	BLOCK_LIST    BlockType = "list"
	BLOCK_TABLE   BlockType = "table"
)

type BlockServicer interface {
	Create(ctx context.Context, args CreateBlockArgs) (database.CreateBlockRow, error)
	Delete(ctx context.Context, id string) error
	GetBlocksByPage(ctx context.Context, id string) ([]database.GetBlocksByPageRow, error)
	Restore(ctx context.Context, id string) error
	Update(ctx context.Context, args UpdateBlockArgs) (database.UpdateBlockRow, error)
	Reorder(ctx context.Context, args []ReorderBlockArgs) error
}

type BlockService struct {
	models *database.Queries
	pool   *pgxpool.Pool
}

type CreateBlockArgs struct {
	PageID   string
	ParentID string
	Type     string
	Content  []byte
}

func (b *BlockService) Create(ctx context.Context, args CreateBlockArgs) (database.CreateBlockRow, error) {
	cleanPageId, err := parseUUID(args.PageID)
	if err != nil {
		return database.CreateBlockRow{}, err
	}

	var cleanParentId pgtype.UUID
	if args.ParentID != "" {
		cleanParentId, err = parseUUID(args.ParentID)
		if err != nil {
			return database.CreateBlockRow{}, err
		}
	}

	switch BlockType(args.Type) {
	case BLOCK_TEXT, BLOCK_HEADING, BLOCK_LIST, BLOCK_TABLE:
	default:
		return database.CreateBlockRow{}, ErrBlockKindMissmatch
	}

	order, err := b.getOrder(ctx, cleanPageId, cleanParentId)
	if err != nil {
		return database.CreateBlockRow{}, err
	}

	params := database.CreateBlockParams{
		PageID:        cleanPageId,
		ParentBlockID: cleanParentId,
		Type:          args.Type,
		Content:       args.Content,
		Order:         order + 1,
	}

	return b.models.CreateBlock(ctx, params)
}

func (b *BlockService) Delete(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return b.models.DeleteBlock(ctx, cleanId)
}

func (b *BlockService) Restore(ctx context.Context, id string) error {
	cleanId, err := parseUUID(id)
	if err != nil {
		return err
	}

	return b.models.RestoreBlock(ctx, cleanId)
}

func (b *BlockService) GetBlocksByPage(ctx context.Context, id string) ([]database.GetBlocksByPageRow, error) {
	cleanId, err := parseUUID(id)
	if err != nil {
		return []database.GetBlocksByPageRow{}, err
	}

	return b.models.GetBlocksByPage(ctx, cleanId)
}

type UpdateBlockArgs struct {
	Id      string
	Type    string
	Content []byte
}

func (b *BlockService) Update(ctx context.Context, args UpdateBlockArgs) (database.UpdateBlockRow, error) {
	cleanId, err := parseUUID(args.Id)
	if err != nil {
		return database.UpdateBlockRow{}, err
	}

	switch BlockType(args.Type) {
	case BLOCK_TEXT, BLOCK_HEADING, BLOCK_LIST, BLOCK_TABLE:
	default:
		return database.UpdateBlockRow{}, ErrBlockKindMissmatch
	}

	params := database.UpdateBlockParams{
		Type:    args.Type,
		Content: args.Content,
		ID:      cleanId,
	}
	return b.models.UpdateBlock(ctx, params)
}

type ReorderBlockArgs struct {
	ID    string
	Order int32
}

func (b *BlockService) Reorder(ctx context.Context, args []ReorderBlockArgs) error {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := database.New(tx)

	for _, arg := range args {
		cleanId, err := parseUUID(arg.ID)
		if err != nil {
			return err
		}

		err = q.ReorderBlocks(ctx, database.ReorderBlocksParams{
			NewOrder: arg.Order,
			ID:       cleanId,
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (b *BlockService) getOrder(ctx context.Context, pageId, parentId pgtype.UUID) (int32, error) {
	var order int32

	if !parentId.Valid {
		blocksOnRoot, err := b.models.GetRootBlocksByPage(ctx, pageId)
		if err != nil {
			return -1, err
		}

		for _, v := range blocksOnRoot {
			if order < v.Order {
				order = v.Order
			}
		}
	} else {
		blocksOnParent, err := b.models.GetBlocksByParent(ctx, parentId)
		if err != nil {
			return -1, err
		}

		for _, v := range blocksOnParent {
			if order < v.Order {
				order = v.Order
			}
		}
	}

	return order, nil
}
