package main

import (
	"context"
	"log"
	"strings"

	"github.com/Aergiaaa/noted/internal/database/migrations"
	"github.com/Aergiaaa/noted/internal/env"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := pgxpool.ParseConfig(env.GetEnvString("DB_URL", ""))
	if err != nil {
		log.Fatalf("error parsing database config: %v", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}
	defer pool.Close()

	if err := migrate(context.Background(), pool); err != nil {
		log.Fatalf("migrate error: %v", err)
	}
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz DEFAULT now())")
	if err != nil {
		return err
	}

	for _, name := range migrationFiles() {
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		sql, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}

		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return err
		}

		if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			return err
		}

		log.Printf("applied %s", name)
	}

	return nil
}

func migrationFiles() []string {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		log.Fatalf("error reading migrations: %v", err)
	}

	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}

	return files
}
