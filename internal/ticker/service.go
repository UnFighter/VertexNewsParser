package ticker

import (
	"log"
	"time"
)

type TickerService struct {
	client *MOEXClient
	repo   *TickerRepository
}

func NewTickerService(client *MOEXClient, repo *TickerRepository) *TickerService {
	return &TickerService{
		client: client,
		repo:   repo,
	}
}

// UpdateSecurities — обновляет справочник акций
func (s *TickerService) UpdateSecurities() error {
	securities := []Security{
		{Ticker: "SBER", Name: "Сбербанк", ShortName: "Сбер", Sector: "Банки"},
		{Ticker: "GAZP", Name: "Газпром", ShortName: "Газпром", Sector: "Нефтегаз"},
		{Ticker: "YNDX", Name: "Яндекс", ShortName: "Яндекс", Sector: "IT"},
		{Ticker: "LKOH", Name: "Лукойл", ShortName: "Лукойл", Sector: "Нефтегаз"},
		{Ticker: "ROSN", Name: "Роснефть", ShortName: "Роснефть", Sector: "Нефтегаз"},
		{Ticker: "VTBR", Name: "ВТБ", ShortName: "ВТБ", Sector: "Банки"},
		{Ticker: "GMKN", Name: "Норильский Никель", ShortName: "Норникель", Sector: "Металлургия"},
	}

	return s.repo.SaveSecurities(securities)
}

// UpdateCurrentPrices — обновляет текущие котировки
func (s *TickerService) UpdateCurrentPrices() error {
	data, err := s.client.GetCurrentMarketData()
	if err != nil {
		return err
	}

	return s.repo.SaveCurrentPrices(data)
}

// LoadHistoricalCandles — загрузить исторические данные
func (s *TickerService) LoadHistoricalCandles(ticker string, days int, interval int) error {
	from := time.Now().AddDate(0, 0, -days)
	till := time.Now()

	candles, err := s.client.GetCandles(ticker, interval, from, till)
	if err != nil {
		return err
	}

	return s.repo.SaveCandles(candles)
}

// StartPeriodicUpdate — запускает периодическое обновление текущих котировок
func (s *TickerService) StartPeriodicUpdate(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("🔄 Обновляем текущие котировки MOEX...")
		if err := s.UpdateCurrentPrices(); err != nil {
			log.Printf("❌ Ошибка обновления котировок: %v", err)
		} else {
			log.Println("✅ Текущие котировки успешно обновлены")
		}
	}
}
