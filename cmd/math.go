// Демонстрация расчёта влияния новостей на акции.
// Запуск: go run ./cmd/math.go
package main

import (
	"fmt"
	"time"
	"vertexNewsParser/internal/impact"
)

func main() {
	input1 := impact.NewsInput{
		Sentiment:      0.92,
		MentionScore:   1.0,
		Magnitude:      0.88,
		VolumeRatio:    5.4,
		AbnormalReturn: 0.094,
		SourceAuth:     0.95,
		FactScore:      0.85,
		Novelty:        0.9,
		TimingFactor:   1.45,
		KeywordScore:   0.7,
		PriceChangePct: 9.4,
		PublishedAt:    time.Now(),
		Ticker:         "SBER",
	}

	r1 := impact.CalculateNewsImpact(input1)
	fmt.Printf("=== Пример 1: Сильная позитивная новость ===\n")
	fmt.Printf("Impact: %.3f (%s, %s)\n", r1.Impact, r1.Direction, r1.Strength)
	fmt.Printf("Quality: %.2f | Market Response: %.2f\n\n", r1.QualityScore, r1.MarketResponse)

	input2 := impact.NewsInput{
		Sentiment:      -0.75,
		MentionScore:   1.2,
		Magnitude:      0.65,
		VolumeRatio:    2.8,
		AbnormalReturn: -0.042,
		SourceAuth:     0.8,
		FactScore:      0.9,
		Novelty:        0.75,
		TimingFactor:   1.0,
		KeywordScore:   -0.6,
		PriceChangePct: -4.1,
		PublishedAt:    time.Now(),
		Ticker:         "GAZP",
	}

	r2 := impact.CalculateNewsImpact(input2)
	fmt.Printf("=== Пример 2: Негативная новость ===\n")
	fmt.Printf("Impact: %.3f (%s, %s)\n", r2.Impact, r2.Direction, r2.Strength)
	fmt.Printf("Quality: %.2f | Market Response: %.2f\n", r2.QualityScore, r2.MarketResponse)
}
