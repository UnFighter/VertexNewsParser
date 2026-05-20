package impact

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Run вычисляет влияние всех новых новостей за последние window на все активные тикеры.
func (s *Service) Run(ctx context.Context, window time.Duration) error {
	since := time.Now().Add(-window)

	news, err := s.fetchNews(ctx, since)
	if err != nil {
		return fmt.Errorf("fetch news: %w", err)
	}
	if len(news) == 0 {
		return nil
	}

	metrics, err := s.fetchMarketMetrics(ctx)
	if err != nil {
		return fmt.Errorf("fetch market metrics: %w", err)
	}
	if len(metrics) == 0 {
		log.Println("impact: market data not yet available, skipping")
		return nil
	}

	records := make([]NewsImpactRecord, 0, len(news)*len(metrics))
	for _, n := range news {
		sentiment, mentionScore, magnitude, factScore, keywordScore := scoreText(n.Title, n.Description)
		for _, m := range metrics {
			input := NewsInput{
				Sentiment:      sentiment,
				MentionScore:   mentionScore,
				Magnitude:      magnitude,
				VolumeRatio:    m.VolumeRatio,
				AbnormalReturn: m.ChangePercent / 100.0,
				SourceAuth:     sourceAuthority(n.SourceName),
				FactScore:      factScore,
				Novelty:        noveltyScore(n.PublishedAt),
				TimingFactor:   timingFactor(n.PublishedAt),
				KeywordScore:   keywordScore,
				PriceChangePct: m.ChangePercent,
				PublishedAt:    n.PublishedAt,
				Ticker:         m.Ticker,
			}
			result := CalculateNewsImpact(input)
			records = append(records, NewsImpactRecord{
				NewsID:         n.ID,
				Ticker:         m.Ticker,
				Impact:         result.Impact,
				Direction:      result.Direction,
				Strength:       result.Strength,
				QualityScore:   result.QualityScore,
				MarketResponse: result.MarketResponse,
				Sentiment:      sentiment,
			})
		}
	}

	if err := s.saveImpacts(ctx, records); err != nil {
		return fmt.Errorf("save impacts: %w", err)
	}

	log.Printf("impact: saved %d records (%d news × %d tickers)", len(records), len(news), len(metrics))
	return nil
}

func (s *Service) fetchNews(ctx context.Context, since time.Time) ([]newsItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.source_name, n.title, COALESCE(n.description, ''), n.published_at
		FROM news n
		WHERE n.published_at >= $1
		  AND NOT EXISTS (SELECT 1 FROM news_impact ni WHERE ni.news_id = n.id)
		ORDER BY n.published_at DESC`,
		since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []newsItem
	for rows.Next() {
		var n newsItem
		if err := rows.Scan(&n.ID, &n.SourceName, &n.Title, &n.Description, &n.PublishedAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func (s *Service) fetchMarketMetrics(ctx context.Context) ([]marketMetrics, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			cp.ticker,
			cp.change_percent,
			CASE
				WHEN COALESCE(avg_vol.avg_volume, 0) = 0 THEN 1.0
				ELSE cp.volume_today::float / avg_vol.avg_volume
			END AS volume_ratio
		FROM current_prices cp
		LEFT JOIN (
			SELECT ticker, AVG(volume)::float AS avg_volume
			FROM candles
			WHERE interval = 60
			GROUP BY ticker
			HAVING COUNT(*) >= 5
		) avg_vol ON avg_vol.ticker = cp.ticker`)
	if err != nil {
		var pgErr *pgconn.PgError
		// таблица ещё не создана (42P01 = undefined_table)
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			log.Println("impact: market tables not yet initialized")
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var result []marketMetrics
	for rows.Next() {
		var m marketMetrics
		if err := rows.Scan(&m.Ticker, &m.ChangePercent, &m.VolumeRatio); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *Service) saveImpacts(ctx context.Context, records []NewsImpactRecord) error {
	const query = `
		INSERT INTO news_impact
			(news_id, ticker, impact, direction, strength, quality_score, market_response, sentiment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (news_id, ticker) DO UPDATE SET
			impact          = EXCLUDED.impact,
			direction       = EXCLUDED.direction,
			strength        = EXCLUDED.strength,
			quality_score   = EXCLUDED.quality_score,
			market_response = EXCLUDED.market_response,
			sentiment       = EXCLUDED.sentiment,
			calculated_at   = NOW()`

	batch := &pgx.Batch{}
	for _, rec := range records {
		batch.Queue(query,
			rec.NewsID, rec.Ticker, rec.Impact, rec.Direction, rec.Strength,
			rec.QualityScore, rec.MarketResponse, rec.Sentiment,
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch item %d: %w", i, err)
		}
	}
	return nil
}
