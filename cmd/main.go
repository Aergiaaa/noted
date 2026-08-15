package main

import (
	"context"
	"log"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/Aergiaaa/noted/internal/env"
	"github.com/Aergiaaa/noted/service"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type config struct {
	host   string
	port   int
	secret string
	store  *sessions.CookieStore
}

type App struct {
	config

	service *service.Services
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

	config := config{
		host:   env.GetEnvString("HOST", "localhost"),
		port:   env.GetEnvInt("PORT", 3000),
		secret: env.GetEnvString("SECRET", ""),
	}
	config.store = sessions.NewCookieStore([]byte(config.secret))

	app := &App{
		config: config,
	}
	app.service = service.InitServices(database.New(db), db)

	if err := app.serve(); err != nil {
		log.Fatalf("error serving app: %v", err)
	}

}
