package ticker

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

type TickerRepository struct {
	db *sqlx.DB
}

func NewTickerRepository(db *sqlx.DB) *TickerRepository {
	return &TickerRepository{db: db}
}

// SaveSecurities — сохраняет справочник акций
func (r *TickerRepository) SaveSecurities(securities []Security) error {
	query := `
		INSERT INTO securities (ticker, name, short_name, sector)
		VALUES (:ticker, :name, :short_name, :sector)
		ON CONFLICT (ticker) 
		DO UPDATE SET 
			name = EXCLUDED.name,
			short_name = EXCLUDED.short_name,
			sector = EXCLUDED.sector,
			updated_at = NOW()`

	_, err := r.db.NamedExec(query, securities)
	if err != nil {
		return fmt.Errorf("failed to save securities: %w", err)
	}
	return nil
}

// SaveCurrentPrices — батч-обновление всех котировок в одной транзакции
func (r *TickerRepository) SaveCurrentPrices(data []MarketData) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO current_prices
			(ticker, last_price, change, change_percent, volume_today, last_trade_time, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (ticker)
		DO UPDATE SET
			last_price      = EXCLUDED.last_price,
			change          = EXCLUDED.change,
			change_percent  = EXCLUDED.change_percent,
			volume_today    = EXCLUDED.volume_today,
			last_trade_time = EXCLUDED.last_trade_time,
			updated_at      = NOW()`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, item := range data {
		if item.Ticker == "" || item.Last == 0 {
			continue
		}
		if _, err := stmt.Exec(item.Ticker, item.Last, item.Change, item.ChangePercent, item.VolumeToday, item.LastTradeTime); err != nil {
			log.Printf("Ошибка сохранения тикера %s: %v", item.Ticker, err)
		}
	}

	return tx.Commit()
}

// SaveCandles
func (r *TickerRepository) SaveCandles(candles []Candle) error {
	query := `
		INSERT INTO candles (ticker, timestamp, open, high, low, close, volume, interval)
		VALUES (:ticker, :timestamp, :open, :high, :low, :close, :volume, :interval)
		ON CONFLICT (ticker, timestamp, interval) DO NOTHING`

	_, err := r.db.NamedExec(query, candles)
	return err
}
