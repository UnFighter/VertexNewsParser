package ticker

import (
	"strconv"
	"time"
)

func parseMarketData(columns []string, data [][]interface{}) []MarketData {
	var result []MarketData
	colIndex := make(map[string]int)

	for i, col := range columns {
		colIndex[col] = i
	}

	for _, row := range data {
		if len(row) == 0 {
			continue
		}

		md := MarketData{
			Ticker:        getString(row, colIndex["SECID"]),
			Last:          getFloat(row, colIndex["LAST"]),
			Change:        getFloat(row, colIndex["CHANGE"]),
			ChangePercent: getFloat(row, colIndex["CHANGEPERCENT"]),
			VolumeToday:   getInt64(row, colIndex["VALTODAY"]),
			LastTradeTime: getString(row, colIndex["TIME"]),
		}
		result = append(result, md)
	}
	return result
}

func parseCandles(columns []string, data [][]interface{}, ticker string, interval int) []Candle {
	var candles []Candle
	colIndex := make(map[string]int)
	for i, col := range columns {
		colIndex[col] = i
	}

	for _, row := range data {
		if len(row) < 7 {
			continue
		}

		tsStr := getString(row, colIndex["begin"])
		ts, _ := time.Parse("2006-01-02 15:04:05", tsStr)

		candle := Candle{
			Ticker:    ticker,
			Timestamp: ts,
			Open:      getFloat(row, colIndex["open"]),
			High:      getFloat(row, colIndex["high"]),
			Low:       getFloat(row, colIndex["low"]),
			Close:     getFloat(row, colIndex["close"]),
			Volume:    getInt64(row, colIndex["volume"]),
			Interval:  interval,
		}
		candles = append(candles, candle)
	}
	return candles
}

// Helper functions
func getString(row []interface{}, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	if s, ok := row[idx].(string); ok {
		return s
	}
	return ""
}

func getFloat(row []interface{}, idx int) float64 {
	if idx < 0 || idx >= len(row) {
		return 0
	}
	switch v := row[idx].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

func getInt64(row []interface{}, idx int) int64 {
	if idx < 0 || idx >= len(row) {
		return 0
	}
	switch v := row[idx].(type) {
	case float64:
		return int64(v)
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	}
	return 0
}
