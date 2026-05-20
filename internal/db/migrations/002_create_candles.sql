-- +goose Up
CREATE TABLE IF NOT EXISTS candles (
                                       id          BIGSERIAL PRIMARY KEY,
                                       ticker      VARCHAR(20) NOT NULL,
                                       timestamp   TIMESTAMPTZ NOT NULL,
                                       open        DECIMAL(14,6) NOT NULL,
                                       high        DECIMAL(14,6) NOT NULL,
                                       low         DECIMAL(14,6) NOT NULL,
                                       close       DECIMAL(14,6) NOT NULL,
                                       volume      BIGINT NOT NULL DEFAULT 0,
                                       interval    INTEGER NOT NULL,                    -- 1, 5, 10, 60, 1440 (минуты)

                                       CONSTRAINT uk_candles UNIQUE (ticker, timestamp, interval)
);

CREATE INDEX IF NOT EXISTS idx_candles_ticker ON candles(ticker);
CREATE INDEX IF NOT EXISTS idx_candles_timestamp ON candles(timestamp);
CREATE INDEX IF NOT EXISTS idx_candles_ticker_interval ON candles(ticker, interval);
CREATE INDEX IF NOT EXISTS idx_candles_timestamp_interval ON candles(timestamp, interval);

-- +goose Down
DROP TABLE IF EXISTS candles;