package internal

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func loadSourcesFromEnv() []Source {
	var sources []Source

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "SOURCE_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 || parts[1] == "" {
				continue
			}

			key := strings.TrimPrefix(parts[0], "SOURCE_")
			sources = append(sources, Source{
				Name: key,
				URL:  parts[1],
			})
		}
	}
	return sources
}

func SeedSources(ctx context.Context, pool *pgxpool.Pool) {
	sources := loadSourcesFromEnv()
	if len(sources) == 0 {
		log.Println("Warning: no sources found in environment variables (SOURCE_*)")
		return
	}

	for _, s := range sources {
		_, err := pool.Exec(ctx,
			"INSERT INTO sources (name, url, active) VALUES ($1, $2, TRUE) ON CONFLICT (url) DO NOTHING",
			s.Name, s.URL)

		if err != nil {
			log.Printf("Error seeding source %s: %v", s.Name, err)
		} else {
			log.Printf("Source seeded/updated: %s → %s", s.Name, s.URL)
		}
	}
}
