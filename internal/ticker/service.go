package ticker

import (
	"log"
	"sync"
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

// UpdateSecurities — загружает полный справочник акций TQBR с MOEX
func (s *TickerService) UpdateSecurities() error {
	securities, err := s.client.GetAllSecurities()
	if err != nil {
		return err
	}
	if len(securities) == 0 {
		return nil
	}
	log.Printf("ticker: получено %d акций с MOEX", len(securities))
	return s.repo.SaveSecurities(securities)
}

// LoadAllHistoricalCandles — загружает свечи для всех тикеров из securities параллельно
func (s *TickerService) LoadAllHistoricalCandles(days, interval, workers int) error {
	securities, err := s.client.GetAllSecurities()
	if err != nil {
		return err
	}

	tickers := make([]string, 0, len(securities))
	for _, sec := range securities {
		tickers = append(tickers, sec.Ticker)
	}

	log.Printf("ticker: загружаем свечи для %d тикеров (%d дней, интервал %d мин)", len(tickers), days, interval)

	sem := make(chan struct{}, workers) // ограничение параллельных запросов
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed []string

	for _, t := range tickers {
		wg.Add(1)
		go func(ticker string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.LoadHistoricalCandles(ticker, days, interval); err != nil {
				mu.Lock()
				failed = append(failed, ticker)
				mu.Unlock()
				log.Printf("ticker: ошибка свечей %s: %v", ticker, err)
			} else {
				log.Printf("ticker: свечи загружены %s", ticker)
			}
		}(t)
	}

	wg.Wait()

	if len(failed) > 0 {
		log.Printf("ticker: не удалось загрузить %d тикеров: %v", len(failed), failed)
	}
	return nil
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
