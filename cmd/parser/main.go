package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
)

type Source struct {
	ID   int
	Name string
	URL  string
}

var (
	sanitizer  = bluemonday.StrictPolicy()
	httpClient = &http.Client{Timeout: 20 * time.Second}
)

func main() {
	_ = godotenv.Load()
	log.Println("vertexNewsParser starting...")

	ctx := context.Background()
	pool := mustConnectDB(ctx)
	defer pool.Close()

	mustInitDB(ctx, pool)
	seedSources(ctx, pool)

	fp := gofeed.NewParser()
	fp.Client = httpClient

	interval := getEnvDuration("PARSE_INTERVAL", 10*time.Minute)
	log.Printf("Парсер запущен. Интервал: %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runParse(ctx, pool, fp) // первый запуск сразу

	for range ticker.C {
		runParse(ctx, pool, fp)
	}
}

func mustConnectDB(ctx context.Context) *pgxpool.Pool {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "vnp_user"),
		getEnv("DB_PASSWORD", "vnp_secret"),
		getEnv("DB_HOST", "postgres"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "vnp_db"),
	)

	config, _ := pgxpool.ParseConfig(connStr)
	config.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil || pool.Ping(ctx) != nil {
		log.Fatal("DB connection failed")
	}
	log.Println("PostgreSQL подключён")
	return pool
}

func mustInitDB(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sources (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT UNIQUE NOT NULL,
			active BOOLEAN DEFAULT TRUE
		);

		CREATE TABLE IF NOT EXISTS news (
			id            SERIAL PRIMARY KEY,
			source_id     INT REFERENCES sources(id),
			source_name   TEXT NOT NULL,           -- ← добавили
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

func seedSources(ctx context.Context, pool *pgxpool.Pool) {
	sources := []Source{
		{Name: "ТАСС", URL: "https://tass.ru/rss/v2.xml"}, // русская версия!
		{Name: "РИА Новости", URL: "https://ria.ru/export/rss2/index.xml"},
		{Name: "Лента.ру", URL: "https://lenta.ru/rss/news"},
		{Name: "МК", URL: "https://www.mk.ru/rss/index.xml"},
		{Name: "Российская газета", URL: "https://rg.ru/xml/index.xml"},
		{Name: "Sputnik", URL: "https://sputniknews.ru/export/rss2/index.xml"},
	}

	for _, s := range sources {
		_, _ = pool.Exec(ctx,
			"INSERT INTO sources (name, url) VALUES ($1, $2) ON CONFLICT (url) DO NOTHING",
			s.Name, s.URL)
	}
}

func runParse(ctx context.Context, pool *pgxpool.Pool, fp *gofeed.Parser) {
	log.Println("Начало парсинга...")

	rows, _ := pool.Query(ctx, "SELECT id, name, url FROM sources WHERE active = TRUE")
	defer rows.Close()

	var wg sync.WaitGroup
	for rows.Next() {
		var src Source
		rows.Scan(&src.ID, &src.Name, &src.URL)

		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			parseSource(ctx, pool, fp, s)
		}(src)
	}
	wg.Wait()
	log.Println("Парсинг завершён")
}

func parseSource(ctx context.Context, pool *pgxpool.Pool, fp *gofeed.Parser, src Source) {
	log.Printf("[%s] Загрузка...", src.Name)

	feed, err := fp.ParseURL(src.URL)
	if err != nil {
		log.Printf("[%s] Ошибка: %v", src.Name, err)
		return
	}

	batch := &pgx.Batch{}

	for _, item := range feed.Items {
		title := strings.TrimSpace(item.Title)
		if title == "" || len(title) < 10 {
			continue
		}

		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}

		desc := ""
		if item.Description != "" {
			desc = item.Description
		} else if item.Content != "" {
			desc = item.Content
		}
		desc = sanitizeAndTruncate(desc, 600)

		raw, _ := json.Marshal(item)
		publishedAt := time.Now().UTC()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		batch.Queue(`
			INSERT INTO news (
				source_id, source_name, title, link, description, 
				raw_json, title_hash, published_at, categories
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (title_hash) DO NOTHING`,
			src.ID, src.Name, title, link, desc, raw, hashTitle(title), publishedAt, item.Categories)
	}

	if batch.Len() > 0 {
		br := pool.SendBatch(ctx, batch)
		_ = br.Close()
	}

	log.Printf("[%s] Добавлено: %d новостей", src.Name, batch.Len())
}

func hashTitle(title string) string {
	h := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(title))))
	return hex.EncodeToString(h[:])
}

func sanitizeAndTruncate(s string, maxLen int) string {
	s = sanitizer.Sanitize(strings.TrimSpace(s))
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
