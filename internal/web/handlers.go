package web

import (
	"encoding/json"
	"log"
	"net/http"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

// GET /api/tickers
type tickerInfo struct {
	Ticker    string `json:"ticker"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	Sector    string `json:"sector"`
}

func (s *Server) handleTickers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(),
		`SELECT ticker, COALESCE(name,''), COALESCE(short_name,''), COALESCE(sector,'')
		 FROM securities WHERE is_active = TRUE ORDER BY ticker`)
	if err != nil {
		// fallback: берём тикеры из котировок
		rows, err = s.pool.Query(r.Context(),
			`SELECT ticker, ticker, ticker, '' FROM current_prices ORDER BY ticker`)
		if err != nil {
			writeJSON(w, []tickerInfo{})
			return
		}
	}
	defer rows.Close()

	result := make([]tickerInfo, 0)
	for rows.Next() {
		var t tickerInfo
		if err := rows.Scan(&t.Ticker, &t.Name, &t.ShortName, &t.Sector); err == nil {
			result = append(result, t)
		}
	}
	writeJSON(w, result)
}

// GET /api/candles?ticker=SBER
type candlePoint struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

func (s *Server) handleCandles(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker required", http.StatusBadRequest)
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT EXTRACT(EPOCH FROM timestamp)::bigint, open, high, low, close, volume
		FROM candles
		WHERE ticker = $1 AND interval = 60
		ORDER BY timestamp ASC
		LIMIT 720`, // ~30 дней часовых свечей
		ticker,
	)
	if err != nil {
		writeJSON(w, []candlePoint{})
		return
	}
	defer rows.Close()

	result := make([]candlePoint, 0)
	for rows.Next() {
		var c candlePoint
		if err := rows.Scan(&c.Time, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err == nil {
			result = append(result, c)
		}
	}
	writeJSON(w, result)
}

// GET /api/current-price?ticker=SBER
type currentPrice struct {
	Ticker        string  `json:"ticker"`
	LastPrice     float64 `json:"last_price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	VolumeToday   int64   `json:"volume_today"`
}

func (s *Server) handleCurrentPrice(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker required", http.StatusBadRequest)
		return
	}

	var p currentPrice
	err := s.pool.QueryRow(r.Context(), `
		SELECT ticker,
		       COALESCE(last_price, 0),
		       COALESCE(change, 0),
		       COALESCE(change_percent, 0),
		       COALESCE(volume_today, 0)
		FROM current_prices WHERE ticker = $1`,
		ticker,
	).Scan(&p.Ticker, &p.LastPrice, &p.Change, &p.ChangePercent, &p.VolumeToday)
	if err != nil {
		writeJSON(w, nil)
		return
	}
	writeJSON(w, p)
}

// GET /api/news-impact?ticker=SBER
type newsImpactItem struct {
	NewsID      int     `json:"news_id"`
	Title       string  `json:"title"`
	Link        string  `json:"link"`
	SourceName  string  `json:"source_name"`
	PublishedAt int64   `json:"published_at"`
	Impact      float64 `json:"impact"`
	Direction   string  `json:"direction"`
	Strength    string  `json:"strength"`
	Sentiment   float64 `json:"sentiment"`
}

func (s *Server) handleNewsImpact(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker required", http.StatusBadRequest)
		return
	}

	// LEFT JOIN: показываем новости даже если news_impact ещё не рассчитан
	rows, err := s.pool.Query(r.Context(), `
		SELECT
			n.id,
			n.title,
			COALESCE(n.link, ''),
			n.source_name,
			EXTRACT(EPOCH FROM n.published_at)::bigint,
			COALESCE(ni.impact, 0),
			COALESCE(ni.direction, 'neutral'),
			COALESCE(ni.strength, 'weak'),
			COALESCE(ni.sentiment, 0)
		FROM news n
		LEFT JOIN news_impact ni ON ni.news_id = n.id AND ni.ticker = $1
		WHERE n.published_at >= NOW() - INTERVAL '7 days'
		ORDER BY
			CASE WHEN ni.impact IS NOT NULL THEN ABS(ni.impact) ELSE -1 END DESC,
			n.published_at DESC
		LIMIT 50`,
		ticker,
	)
	if err != nil {
		writeJSON(w, []newsImpactItem{})
		return
	}
	defer rows.Close()

	result := make([]newsImpactItem, 0)
	for rows.Next() {
		var item newsImpactItem
		err := rows.Scan(
			&item.NewsID, &item.Title, &item.Link, &item.SourceName,
			&item.PublishedAt, &item.Impact, &item.Direction, &item.Strength, &item.Sentiment,
		)
		if err == nil {
			result = append(result, item)
		}
	}
	writeJSON(w, result)
}
