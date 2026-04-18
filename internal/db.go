package internal

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MustConnectDB(ctx context.Context) *pgxpool.Pool {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "vnp_user"),
		getEnv("DB_PASSWORD", "vnp_secret"),
		getEnv("DB_HOST", "postgres"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "vnp_db"),
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Fatal("Failed to parse DB config:", err)
	}
	config.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("DB ping failed:", err)
	}

	log.Println("PostgreSQL connected successfully")
	return pool
}

func MustInitDB(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sources (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT UNIQUE NOT NULL,
			active BOOLEAN DEFAULT TRUE
		);

		CREATE TABLE IF NOT EXISTS news (
			id            SERIAL PRIMARY KEY,
			source_id     INT REFERENCES sources(id) ON DELETE CASCADE,
			source_name   TEXT NOT NULL,
			title         TEXT NOT NULL,
			link          TEXT NOT NULL,
			description   TEXT,
			raw_json      JSONB,
			title_hash    TEXT UNIQUE,
			published_at  TIMESTAMPTZ,
			fetched_at    TIMESTAMPTZ DEFAULT NOW(),
			categories    TEXT[],
			created_at    TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_news_published ON news(published_at DESC);
		CREATE INDEX IF NOT EXISTS idx_news_hash ON news(title_hash);
	`)
	if err != nil {
		log.Fatal("DB init error:", err)
	}
}
