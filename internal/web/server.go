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
	s.mux.Handle("/", http.FileServer(staticFS))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
