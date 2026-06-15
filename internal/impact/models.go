package impact

import "time"

type NewsInput struct {
	Sentiment      float64
	MentionScore   float64
	Magnitude      float64
	VolumeRatio    float64
	AbnormalReturn float64
	SourceAuth     float64
	FactScore      float64
	Novelty        float64
	TimingFactor   float64
	KeywordScore   float64
	PriceChangePct float64
	PublishedAt    time.Time
	Ticker         string
}

type NewsImpactResult struct {
	Impact         float64
	QualityScore   float64
	MarketResponse float64
	Direction      string
	Strength       string
}

type NewsImpactRecord struct {
	NewsID          int
	Ticker          string
	Impact          float64
	Direction       string
	Strength        string
	QualityScore    float64
	MarketResponse  float64
	Sentiment       float64
	TickerMentioned bool
}

type newsItem struct {
	ID          int
	SourceName  string
	Title       string
	Description string
	PublishedAt time.Time
}

type marketMetrics struct {
	Ticker        string
	ChangePercent float64
	VolumeRatio   float64
	ShortName     string
	Name          string
}
