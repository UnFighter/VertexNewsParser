package ticker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

type MOEXClient struct {
	client *resty.Client
}

func NewMOEXClient() *MOEXClient {
	c := resty.New().
		SetBaseURL("https://iss.moex.com/iss").
		SetTimeout(20 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second)

	return &MOEXClient{client: c}
}

// GetCurrentMarketData — текущие котировки по всем акциям
func (c *MOEXClient) GetCurrentMarketData() ([]MarketData, error) {
	resp, err := c.client.R().Get("/engines/stock/markets/shares/boards/TQBR/securities.json")
	if err != nil {
		return nil, err
	}

	var result struct {
		Marketdata struct {
			Columns []string        `json:"columns"`
			Data    [][]interface{} `json:"data"`
		} `json:"marketdata"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}

	return parseMarketData(result.Marketdata.Columns, result.Marketdata.Data), nil
}

// GetCandles — исторические свечи
func (c *MOEXClient) GetCandles(ticker string, interval int, from, till time.Time) ([]Candle, error) {
	url := fmt.Sprintf("/engines/stock/markets/shares/boards/TQBR/securities/%s/candles.json", ticker)

	resp, err := c.client.R().
		SetQueryParams(map[string]string{
			"from":     from.Format("2006-01-02"),
			"till":     till.Format("2006-01-02"),
			"interval": strconv.Itoa(interval),
		}).
		Get(url)

	if err != nil {
		return nil, err
	}

	var respData MOEXCandlesResponse
	if err := json.Unmarshal(resp.Body(), &respData); err != nil {
		return nil, err
	}

	return parseCandles(respData.Candles.Columns, respData.Candles.Data, ticker, interval), nil
}
