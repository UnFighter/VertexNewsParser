package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
)

type NewsSource struct {
	Name string
	URL  string
}

var sources = []NewsSource{
	{"ТАСС", "https://tass.com/rss/v2.xml"},
	{"РИА Новости", "https://ria.ru/export/rss2/index.xml"},
	{"Лента.ру", "https://lenta.ru/rss/news"},
	{"МК", "https://www.mk.ru/rss/index.xml"},
	{"Российская газета", "https://rg.ru/xml/index.xml"},
}

func main() {
	db := connectDB()
	defer db.Close()

	createTable(db)

	fp := gofeed.NewParser()

	ticker := time.NewTicker(getParseInterval())
	defer ticker.Stop()

	log.Println("Парсер новостей запущен")

	runParse(db, fp)

	for range ticker.C {
		runParse(db, fp)
	}
}

func connectDB() *sql.DB {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "postgres"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "vnp_user"),
		getEnv("DB_PASSWORD", "vnp_secret"),
		getEnv("DB_NAME", "vnp_db"),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Не удалось открыть соединение с БД:", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("Не удалось подключиться к PostgreSQL:", err)
	}

	log.Println("Подключение к PostgreSQL успешно")
	return db
}

func createTable(db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS news (
			id            SERIAL PRIMARY KEY,
			source        TEXT NOT NULL,
			title         TEXT NOT NULL,
			link          TEXT UNIQUE NOT NULL,
			description   TEXT,
			published_at  TIMESTAMPTZ,
			created_at    TIMESTAMPTZ DEFAULT NOW()
		);
	`)
	if err != nil {
		log.Fatal("Не удалось создать таблицу:", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getParseInterval() time.Duration {
	interval := os.Getenv("PARSE_INTERVAL")
	if interval == "" {
		return 10 * time.Minute
	}
	if d, err := time.ParseDuration(interval); err == nil {
		return d
	}
	return 10 * time.Minute
}

func runParse(db *sql.DB, fp *gofeed.Parser) {
	log.Println("Начало парсинга...")

	for _, src := range sources {
		log.Printf("[%s] Загрузка фида...", src.Name)

		feed, err := fp.ParseURL(src.URL)
		if err != nil {
			log.Printf("[%s] Ошибка: %v", src.Name, err)
			continue
		}

		inserted := 0
		for _, item := range feed.Items {
			title := strings.TrimSpace(item.Title)
			link := strings.TrimSpace(item.Link)
			if title == "" || link == "" {
				continue
			}

			desc := getDescription(item)

			publishedAt := time.Now()
			if item.PublishedParsed != nil {
				publishedAt = *item.PublishedParsed
			}

			_, err := db.Exec(`
				INSERT INTO news (source, title, link, description, published_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (link) DO NOTHING`,
				src.Name, title, link, desc, publishedAt)

			if err == nil {
				inserted++
			}
		}

		log.Printf("[%s] Добавлено новых: %d (всего в фиде: %d)", src.Name, inserted, len(feed.Items))
	}

	log.Println("Парсинг завершён")
}

func getDescription(item *gofeed.Item) string {
	if item.Description != "" {
		return truncate(strings.TrimSpace(item.Description), 500)
	}
	if item.Content != "" {
		return truncate(strings.TrimSpace(item.Content), 500)
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}
