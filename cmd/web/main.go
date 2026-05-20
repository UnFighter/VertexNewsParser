package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"vertexNewsParser/internal/news"
	"vertexNewsParser/internal/web"

	"github.com/joho/godotenv"
)

//go:embed static
var staticFiles embed.FS

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using environment variables")
	}

	ctx := context.Background()
	pool := news.MustConnectDB(ctx)
	defer pool.Close()

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	srv := web.NewServer(pool, http.FS(staticFS))

	port := os.Getenv("WEB_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Web server: http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatal(err)
	}
}
