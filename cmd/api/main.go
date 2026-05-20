package main

import (
	"fmt"
	"log"
	"os"
	"time"
	"vertexNewsParser/internal/db"
	"vertexNewsParser/internal/ticker"
)

func main() {
	log.Println("🚀 Запуск приложения Vertex News Parser...")

	database, err := db.NewConnection(getDSN())
	if err != nil {
		log.Fatalf("❌ Не удалось подключиться к БД: %v", err)
	}
	defer database.Close()

	log.Println("✅ Подключение к PostgreSQL успешно")

	if err := db.Migrate(database); err != nil {
		log.Fatalf("❌ Ошибка миграций: %v", err)
	}

	// Инициализация сервиса тикеров
	tickerClient := ticker.NewMOEXClient()
	tickerRepo := ticker.NewTickerRepository(database)
	tickerService := ticker.NewTickerService(tickerClient, tickerRepo)

	log.Println("📊 Заполняем справочник акций (securities)...")
	if err := tickerService.UpdateSecurities(); err != nil {
		log.Printf("⚠️ Не удалось обновить справочник акций: %v", err)
	} else {
		log.Println("✅ Справочник акций обновлён")
	}

	log.Println("📈 Загружаем исторические данные...")
	for _, t := range []string{"SBER", "GAZP", "YNDX", "LKOH", "ROSN", "VTBR", "GMKN"} {
		if err := tickerService.LoadHistoricalCandles(t, 30, 60); err != nil {
			log.Printf("⚠️ Ошибка загрузки истории %s: %v", t, err)
		} else {
			log.Printf("✅ Свечи загружены: %s", t)
		}
	}

	log.Println("🔄 Обновляем текущие котировки...")
	if err := tickerService.UpdateCurrentPrices(); err != nil {
		log.Printf("⚠️ Ошибка обновления текущих цен: %v", err)
	} else {
		log.Println("✅ Текущие котировки обновлены")
	}

	// Запускаем периодическое обновление
	go tickerService.StartPeriodicUpdate(5 * time.Minute)

	log.Println("🎉 Приложение работает успешно!")
	select {} // держим программу запущенной
}

func getDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "vnp_user"),
		getEnv("DB_PASSWORD", "vnp_secret"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "vnp_db"),
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
