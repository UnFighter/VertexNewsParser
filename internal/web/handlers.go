package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
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
	// Базируемся на current_prices — там все тикеры с актуальными ценами.
	// Джоиним с securities для получения названий там, где они есть.
	rows, err := s.pool.Query(r.Context(), `
		SELECT
			cp.ticker,
			COALESCE(s.name, cp.ticker),
			COALESCE(s.short_name, cp.ticker),
			COALESCE(s.sector, '')
		FROM current_prices cp
		LEFT JOIN securities s ON s.ticker = cp.ticker
		WHERE cp.last_price > 0
		ORDER BY cp.ticker`)
	if err != nil {
		writeJSON(w, []tickerInfo{})
		return
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

	// 1. Дневные свечи за год из БД
	var result []candlePoint
	rows, err := s.pool.Query(r.Context(), `
		SELECT EXTRACT(EPOCH FROM timestamp)::bigint, open, high, low, close, volume
		FROM candles
		WHERE ticker = $1 AND interval = 24
		  AND timestamp >= NOW() - INTERVAL '1 year'
		ORDER BY timestamp ASC
		LIMIT 400`,
		ticker,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var c candlePoint
			if err := rows.Scan(&c.Time, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err == nil {
				result = append(result, c)
			}
		}
	}

	// 2. Нет дневных в БД — берём часовые из БД (быстро, без внешних запросов)
	if len(result) == 0 {
		rows2, err2 := s.pool.Query(r.Context(), `
			SELECT EXTRACT(EPOCH FROM timestamp)::bigint, open, high, low, close, volume
			FROM candles WHERE ticker = $1 AND interval = 60
			ORDER BY timestamp ASC LIMIT 720`,
			ticker,
		)
		if err2 == nil {
			defer rows2.Close()
			for rows2.Next() {
				var c candlePoint
				if err2 := rows2.Scan(&c.Time, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err2 == nil {
					result = append(result, c)
				}
			}
		}
	}

	// 3. Нет данных в БД вообще — последний шанс: MOEX ISS (медленно, внешний запрос)
	if len(result) == 0 {
		result = fetchCandlesMOEX(ticker, 24, 365)
	}

	writeJSON(w, result)
}

const moexCandleURL = "https://iss.moex.com/iss/engines/stock/markets/shares/boards/TQBR/securities/%s/candles.json"

// fetchCandlesMOEX — получает свечи с MOEX ISS с пагинацией.
// interval: 24=день, 60=час. days: сколько дней истории запросить.
func fetchCandlesMOEX(ticker string, interval, days int) []candlePoint {
	from := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	till := time.Now().Format("2006-01-02")
	apiURL := fmt.Sprintf(moexCandleURL, ticker)
	httpClient := &http.Client{Timeout: 8 * time.Second}

	var result []candlePoint
	const pageSize = 500

	for start := 0; ; start += pageSize {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
		if err != nil {
			break
		}
		q := req.URL.Query()
		q.Set("from", from)
		q.Set("till", till)
		q.Set("interval", strconv.Itoa(interval))
		q.Set("start", strconv.Itoa(start))
		req.URL.RawQuery = q.Encode()

		resp, err := httpClient.Do(req)
		if err != nil {
			break
		}

		var body struct {
			Candles struct {
				Columns []string        `json:"columns"`
				Data    [][]interface{} `json:"data"`
			} `json:"candles"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if decErr != nil || len(body.Candles.Data) == 0 {
			break
		}

		colIdx := make(map[string]int, len(body.Candles.Columns))
		for i, c := range body.Candles.Columns {
			colIdx[c] = i
		}

		for _, row := range body.Candles.Data {
			tsStr, _ := row[colIdx["begin"]].(string)
			ts, err := time.ParseInLocation("2006-01-02 15:04:05", tsStr, time.UTC)
			if err != nil {
				continue
			}
			result = append(result, candlePoint{
				Time:   ts.Unix(),
				Open:   toFloat(row[colIdx["open"]]),
				High:   toFloat(row[colIdx["high"]]),
				Low:    toFloat(row[colIdx["low"]]),
				Close:  toFloat(row[colIdx["close"]]),
				Volume: int64(toFloat(row[colIdx["volume"]])),
			})
		}

		// Если страница неполная — данных больше нет
		if len(body.Candles.Data) < pageSize {
			break
		}
	}
	return result
}

func toFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

// GET /api/prices — все текущие цены с названиями
type priceItem struct {
	Ticker        string  `json:"ticker"`
	ShortName     string  `json:"short_name"`
	Name          string  `json:"name"`
	LastPrice     float64 `json:"last_price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
}

func (s *Server) handlePrices(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT
			cp.ticker,
			COALESCE(s.short_name, cp.ticker),
			COALESCE(s.name, cp.ticker),
			COALESCE(cp.last_price, 0),
			COALESCE(cp.change, 0),
			COALESCE(cp.change_percent, 0)
		FROM current_prices cp
		LEFT JOIN securities s ON s.ticker = cp.ticker
		WHERE cp.last_price > 0
		ORDER BY cp.ticker`)
	if err != nil {
		writeJSON(w, []priceItem{})
		return
	}
	defer rows.Close()

	result := make([]priceItem, 0)
	for rows.Next() {
		var p priceItem
		if err := rows.Scan(&p.Ticker, &p.ShortName, &p.Name, &p.LastPrice, &p.Change, &p.ChangePercent); err == nil {
			result = append(result, p)
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

	rows, err := s.pool.Query(r.Context(), `
		SELECT
			n.id,
			n.title,
			COALESCE(n.link, ''),
			n.source_name,
			EXTRACT(EPOCH FROM n.published_at)::bigint,
			ni.impact,
			ni.direction,
			ni.strength,
			COALESCE(ni.sentiment, 0)
		FROM news n
		JOIN news_impact ni ON ni.news_id = n.id AND ni.ticker = $1
		WHERE n.published_at >= NOW() - INTERVAL '7 days'
		  AND ni.ticker_mentioned = TRUE
		ORDER BY ABS(ni.impact) DESC, n.published_at DESC
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

// GET /api/top-impact — топ акций по суммарному влиянию новостей за последние 7 дней
type topImpactItem struct {
	Ticker        string  `json:"ticker"`
	ShortName     string  `json:"short_name"`
	Name          string  `json:"name"`
	LastPrice     float64 `json:"last_price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	NewsCount     int     `json:"news_count"`
	TotalImpact   float64 `json:"total_impact"`
	NetImpact     float64 `json:"net_impact"`
	AvgSentiment  float64 `json:"avg_sentiment"`
	LatestNewsAt  int64   `json:"latest_news_at"`
}

func (s *Server) handleTopImpact(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT
			ni.ticker,
			COALESCE(s.short_name, ni.ticker),
			COALESCE(s.name, ni.ticker),
			COALESCE(cp.last_price, 0),
			COALESCE(cp.change, 0),
			COALESCE(cp.change_percent, 0),
			COUNT(DISTINCT n.id)::int,
			SUM(ABS(ni.impact)),
			SUM(ni.impact),
			AVG(ni.sentiment),
			EXTRACT(EPOCH FROM MAX(n.published_at))::bigint
		FROM news_impact ni
		JOIN news n ON n.id = ni.news_id
		LEFT JOIN securities s ON s.ticker = ni.ticker
		LEFT JOIN current_prices cp ON cp.ticker = ni.ticker
		WHERE n.published_at >= NOW() - INTERVAL '7 days'
		  AND ni.ticker_mentioned = TRUE
		GROUP BY ni.ticker, s.short_name, s.name, cp.last_price, cp.change, cp.change_percent
		HAVING COUNT(DISTINCT n.id) > 0
		ORDER BY SUM(ABS(ni.impact)) DESC
		LIMIT 10`)
	if err != nil {
		writeJSON(w, []topImpactItem{})
		return
	}
	defer rows.Close()

	result := make([]topImpactItem, 0)
	for rows.Next() {
		var item topImpactItem
		if err := rows.Scan(
			&item.Ticker, &item.ShortName, &item.Name,
			&item.LastPrice, &item.Change, &item.ChangePercent,
			&item.NewsCount, &item.TotalImpact, &item.NetImpact,
			&item.AvgSentiment, &item.LatestNewsAt,
		); err == nil {
			result = append(result, item)
		}
	}
	writeJSON(w, result)
}

// ─── Logo ──────────────────────────────────────────────────────────────────────

var (
	logoCache   sync.Map
	logoClient  = &http.Client{Timeout: 8 * time.Second}
	logoPalette = []string{
		"#3B82F6", "#10B981", "#F59E0B", "#EF4444",
		"#8B5CF6", "#EC4899", "#06B6D4", "#84CC16", "#F97316", "#14B8A6",
	}
)

type logoEntry struct {
	data        []byte
	contentType string
}

// GET /api/logo/{ticker}
func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/api/logo/"))
	if ticker == "" {
		http.Error(w, "ticker required", http.StatusBadRequest)
		return
	}
	if cached, ok := logoCache.Load(ticker); ok {
		writeLogo(w, cached.(logoEntry))
		return
	}
	// Отвечаем SVG-аватаром немедленно, реальный логотип загружаем в фоне
	svg := logoEntry{[]byte(svgAvatar(ticker)), "image/svg+xml"}
	writeLogo(w, svg)
	go func() {
		entry := resolveLogo(ticker)
		logoCache.Store(ticker, entry)
	}()
}

func writeLogo(w http.ResponseWriter, e logoEntry) {
	w.Header().Set("Content-Type", e.contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(e.data)
}

func resolveLogo(ticker string) logoEntry {
	if isin := moexISIN(ticker); isin != "" {
		url := fmt.Sprintf("https://invest-brands.cdn-tinkoff.ru/%sx160.png", isin)
		if data, err := fetchLogoBytes(url); err == nil && isPNGData(data) {
			return logoEntry{data, "image/png"}
		}
	}
	return logoEntry{[]byte(svgAvatar(ticker)), "image/svg+xml"}
}

func moexISIN(ticker string) string {
	reqURL := fmt.Sprintf("https://iss.moex.com/iss/securities/%s.json?iss.meta=off&iss.only=description", ticker)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return ""
	}
	resp, err := logoClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var body struct {
		Description struct {
			Columns []string        `json:"columns"`
			Data    [][]interface{} `json:"data"`
		} `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}
	nameIdx, valIdx := -1, -1
	for i, c := range body.Description.Columns {
		if c == "name" {
			nameIdx = i
		}
		if c == "value" {
			valIdx = i
		}
	}
	if nameIdx < 0 || valIdx < 0 {
		return ""
	}
	for _, row := range body.Description.Data {
		if len(row) > nameIdx && len(row) > valIdx {
			if n, ok := row[nameIdx].(string); ok && n == "ISIN" {
				if v, ok := row[valIdx].(string); ok {
					return v
				}
			}
		}
	}
	return ""
}

func fetchLogoBytes(rawURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := logoClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func isPNGData(data []byte) bool {
	return len(data) > 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47
}

func svgAvatar(ticker string) string {
	h := 0
	for _, c := range ticker {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	color := logoPalette[h%len(logoPalette)]
	letter := string([]rune(ticker)[0])
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40"><rect width="40" height="40" rx="8" fill="%s" fill-opacity="0.15"/><text x="20" y="26" text-anchor="middle" font-family="system-ui,-apple-system,sans-serif" font-size="16" font-weight="700" fill="%s">%s</text></svg>`,
		color, color, letter,
	)
}

// GET /api/news/general — новости, не привязанные к конкретной акции
type generalNewsItem struct {
	NewsID      int     `json:"news_id"`
	Title       string  `json:"title"`
	Link        string  `json:"link"`
	SourceName  string  `json:"source_name"`
	PublishedAt int64   `json:"published_at"`
	Sentiment   float64 `json:"sentiment"`
}

func (s *Server) handleGeneralNews(w http.ResponseWriter, r *http.Request) {
	// Общеэкономические новости: те, что не упоминают ни один тикер или название компании.
	rows, err := s.pool.Query(r.Context(), `
		SELECT
			n.id,
			n.title,
			COALESCE(n.link, ''),
			n.source_name,
			EXTRACT(EPOCH FROM n.published_at)::bigint,
			COALESCE(
				(SELECT AVG(ni2.sentiment) FROM news_impact ni2 WHERE ni2.news_id = n.id),
				0
			) AS avg_sentiment
		FROM news n
		WHERE n.published_at >= NOW() - INTERVAL '7 days'
		  AND NOT EXISTS (
		      SELECT 1 FROM news_impact ni2
		      WHERE ni2.news_id = n.id AND ni2.ticker_mentioned = TRUE
		  )
		ORDER BY n.published_at DESC
		LIMIT 100`)
	if err != nil {
		writeJSON(w, []generalNewsItem{})
		return
	}
	defer rows.Close()

	result := make([]generalNewsItem, 0)
	for rows.Next() {
		var item generalNewsItem
		if err := rows.Scan(
			&item.NewsID, &item.Title, &item.Link, &item.SourceName,
			&item.PublishedAt, &item.Sentiment,
		); err == nil {
			result = append(result, item)
		}
	}
	writeJSON(w, result)
}
