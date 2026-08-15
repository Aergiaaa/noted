package service

import (
	"github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Services struct {
	models       *database.Queries
	Page        PageServicer
	Block       BlockServicer
	Transaction TransactionServicer
	Pocket      PocketServicer
	Tag         TagServicer
	Taggable    TaggableServicer
	Edge        EdgeServicer
}

func InitServices(models *database.Queries, pool *pgxpool.Pool) *Services {
	return &Services{
		models:       models,
		Page:        &PageService{models: models},
		Block:       &BlockService{models: models, pool: pool},
		Transaction: &TransactionService{models: models},
		Pocket:      &PocketService{models: models},
		Tag:         &TagService{models: models},
		Taggable:    &TaggableService{models: models},
		Edge:        &EdgeService{models: models},
	}
}
