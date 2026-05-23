package main

import (
	"context"
	"log"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/internal/env"
	"github.com/Aergiaaa/noted/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type app struct {
	host    string
	port    int
	service *service.Services
	models  *database.Queries
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	db, err := pgxpool.New(context.Background(), env.GetEnvString("DB_URL", ""))
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}

	app := &app{
		host:   env.GetEnvString("HOST", "localhost"),
		port:   env.GetEnvInt("PORT", 3000),
		models: database.New(db),
	}

	app.service = service.InitServices(app.models, db)

	if err := app.serve(); err != nil {
		log.Fatalf("error serving app: %v", err)
	}

}
