package news

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
)

const maxConcurrentSources = 10

func NewFeedParser() *gofeed.Parser {
	fp := gofeed.NewParser()
	fp.Client = httpClient
	return fp
}

func RunParse(ctx context.Context, pool *pgxpool.Pool, fp *gofeed.Parser) {
	log.Println("Starting parsing cycle...")

	rows, err := pool.Query(ctx, "SELECT id, name, url FROM sources WHERE active = TRUE")
	if err != nil {
		log.Printf("Failed to query sources: %v", err)
		return
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.Name, &src.URL); err != nil {
			log.Printf("Failed to scan source row: %v", err)
			continue
		}
		sources = append(sources, src)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error during rows iteration: %v", err)
	}

	log.Printf("Found %d active sources", len(sources))

	sem := make(chan struct{}, maxConcurrentSources)
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		sem <- struct{}{}
		go func(s Source) {
			defer wg.Done()
			defer func() { <-sem }()
			parseSource(ctx, pool, fp, s)
		}(src)
	}
	wg.Wait()

	log.Println("Parsing cycle completed")
}

func parseSource(ctx context.Context, pool *pgxpool.Pool, fp *gofeed.Parser, src Source) {
	log.Printf("Parsing source: %s (%s)", src.Name, src.URL)

	feed, err := fp.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		log.Printf("Failed to parse %s: %v", src.URL, err)
		return
	}

	const query = `
		INSERT INTO news (source_id, source_name, title, link, description, raw_json, title_hash, published_at, categories)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (title_hash) DO NOTHING`

	batch := &pgx.Batch{}
	for _, item := range feed.Items {
		if item.Title == "" {
			continue
		}

		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		desc := ""
		if item.Description != "" {
			desc = sanitizeAndTruncate(item.Description, 500)
		}

		batch.Queue(query,
			src.ID,
			src.Name,
			sanitizeAndTruncate(item.Title, 500),
			item.Link,
			desc,
			item,
			hashTitle(item.Title),
			publishedAt,
			item.Categories,
		)
	}

	if batch.Len() == 0 {
		log.Printf("Finished %s → 0 items", src.Name)
		return
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	saved := 0
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			log.Printf("Error saving item from %s: %v", src.Name, err)
		} else {
			saved++
		}
	}

	log.Printf("Finished %s → %d/%d items saved", src.Name, saved, batch.Len())
}
