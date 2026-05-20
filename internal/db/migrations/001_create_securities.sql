-- +goose Up
CREATE TABLE IF NOT EXISTS securities (
                                          id            BIGSERIAL PRIMARY KEY,
                                          ticker        VARCHAR(20) UNIQUE NOT NULL,
    name          TEXT,
    short_name    TEXT,
    isin          VARCHAR(20),
    sector        VARCHAR(100),
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS idx_securities_ticker ON securities(ticker);
CREATE INDEX IF NOT EXISTS idx_securities_sector ON securities(sector);

-- +goose Down
DROP TABLE IF EXISTS securities;