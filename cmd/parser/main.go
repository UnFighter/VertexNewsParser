package main

import (
	"context"
	"log"
	"time"
	"vertexNewsParser/internal"

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
	pool := internal.MustConnectDB(ctx)
	defer pool.Close()

	internal.MustInitDB(ctx, pool)
	internal.SeedSources(ctx, pool) // загружает из env при каждом старте

	fp := internal.NewFeedParser()

	interval := internal.GetEnvDuration("PARSE_INTERVAL", 10*time.Minute)
	log.Printf("Parsing interval: %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	internal.RunParse(ctx, pool, fp) // первый запуск сразу

	for range ticker.C {
		internal.RunParse(ctx, pool, fp)
	}
}
