package web

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool *pgxpool.Pool
	mux  *http.ServeMux
}

func NewServer(pool *pgxpool.Pool, staticFS http.FileSystem) *Server {
	s := &Server{pool: pool, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/tickers", s.handleTickers)
	s.mux.HandleFunc("/api/candles", s.handleCandles)
	s.mux.HandleFunc("/api/current-price", s.handleCurrentPrice)
	s.mux.HandleFunc("/api/news-impact", s.handleNewsImpact)
	s.mux.HandleFunc("/api/news/general", s.handleGeneralNews)
	s.mux.HandleFunc("/api/prices", s.handlePrices)
	s.mux.HandleFunc("/api/top-impact", s.handleTopImpact)
	s.mux.HandleFunc("/api/logo/", s.handleLogo)
	s.mux.Handle("/", spaHandler(staticFS))
	return s
}

// spaHandler serves static files when they exist; falls back to index.html for
// all other paths so the JS router handles client-side navigation.
func spaHandler(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
