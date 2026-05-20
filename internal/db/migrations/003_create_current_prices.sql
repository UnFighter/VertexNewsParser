-- +goose Up
CREATE TABLE IF NOT EXISTS current_prices (
                                              id                BIGSERIAL PRIMARY KEY,
                                              ticker            VARCHAR(20) UNIQUE NOT NULL,
                                              last_price        DECIMAL(14,6),
                                              change            DECIMAL(14,6),
                                              change_percent    DECIMAL(10,4),
                                              volume_today      BIGINT,
                                              last_trade_time   TIME,                    -- Только время (HH:MM:SS)
                                              updated_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_current_prices_ticker ON current_prices(ticker);

-- +goose Down
DROP TABLE IF EXISTS current_prices;