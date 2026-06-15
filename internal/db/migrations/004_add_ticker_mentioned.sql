-- +goose Up
ALTER TABLE news_impact ADD COLUMN IF NOT EXISTS ticker_mentioned BOOLEAN NOT NULL DEFAULT TRUE;
CREATE INDEX IF NOT EXISTS idx_news_impact_mentioned ON news_impact(ticker_mentioned);

-- +goose Down
DROP INDEX IF EXISTS idx_news_impact_mentioned;
ALTER TABLE news_impact DROP COLUMN IF EXISTS ticker_mentioned;
