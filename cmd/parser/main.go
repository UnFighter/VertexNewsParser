package main

import (
	"context"
	"log"
	"time"
	"vertexNewsParser/internal/news"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env not loaded, using docker environment variables")
	} else {
		log.Println(".env file loaded successfully")
	}

	log.Println("vertexNewsParser starting...")

	ctx := context.Background()
	pool := news.MustConnectDB(ctx)
	defer pool.Close()

	news.MustInitDB(ctx, pool)
	news.SeedSources(ctx, pool)

	fp := news.NewFeedParser()

	interval := news.GetEnvDuration("PARSE_INTERVAL", 10*time.Minute)
	log.Printf("Parsing interval: %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	news.RunParse(ctx, pool, fp)

	for range ticker.C {
		news.RunParse(ctx, pool, fp)
	}
}
