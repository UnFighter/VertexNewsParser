package ticker

import "time"

type Security struct {
	ID        uint   `db:"id"`
	Ticker    string `db:"ticker"`
	Name      string `db:"name"`
	ShortName string `db:"short_name"`
	ISIN      string `db:"isin"`
	Sector    string `db:"sector"`
	IsActive  bool   `db:"is_active"`
}

type Candle struct {
	ID        uint      `db:"id"`
	Ticker    string    `db:"ticker"`
	Timestamp time.Time `db:"timestamp"`
	Open      float64   `db:"open"`
	High      float64   `db:"high"`
	Low       float64   `db:"low"`
	Close     float64   `db:"close"`
	Volume    int64     `db:"volume"`
	Interval  int       `db:"interval"`
}

type MarketData struct {
	Ticker        string  `db:"ticker"        json:"SECID"`
	Last          float64 `db:"last_price"    json:"LAST"`
	Change        float64 `db:"change"        json:"CHANGE"`
	ChangePercent float64 `db:"change_percent" json:"CHANGEPERCENT"`
	VolumeToday   int64   `db:"volume_today"  json:"VALTODAY"`
	LastTradeTime string  `db:"last_trade_time" json:"TIME"`
}

type MOEXCandlesResponse struct {
	Candles struct {
		Columns []string        `json:"columns"`
		Data    [][]interface{} `json:"data"`
	} `json:"candles"`
}
