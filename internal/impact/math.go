package impact

import "math"

func CalculateNewsImpact(input NewsInput) NewsImpactResult {
	S := math.Max(-1.0, math.Min(1.0, input.Sentiment))
	M := math.Max(0.0, math.Min(1.5, input.MentionScore))
	Mag := math.Max(0.0, math.Min(1.0, input.Magnitude))

	V := (2 / math.Pi) * math.Atan((input.VolumeRatio-1.0)/2.5)
	AR := math.Max(-4.0, math.Min(4.0, input.AbnormalReturn/0.015))

	quality := S * M * Mag *
		(0.7 + 0.3*input.SourceAuth) *
		(0.6 + 0.4*input.FactScore) *
		(0.75 + 0.25*input.KeywordScore) *
		(0.8 + 0.2*input.Novelty)

	interaction := 0.75 * (S * V * Mag)

	marketResp := (1 + 0.45*V) *
		(1 + 0.55*math.Abs(AR)) *
		(1 + 0.22*math.Abs(input.PriceChangePct)/5.0) *
		math.Max(0.6, input.TimingFactor)

	core := 1.2*quality + interaction
	impact := math.Tanh(core) * marketResp

	if math.Abs(S) < 0.12 {
		impact *= 0.35
	}

	return NewsImpactResult{
		Impact:         math.Round(impact*1000) / 1000,
		QualityScore:   math.Round(quality*100) / 100,
		MarketResponse: math.Round(marketResp*100) / 100,
		Direction:      directionLabel(S),
		Strength:       strengthLabel(impact),
	}
}

func directionLabel(s float64) string {
	switch {
	case s > 0.15:
		return "positive"
	case s < -0.15:
		return "negative"
	default:
		return "neutral"
	}
}

func strengthLabel(impact float64) string {
	abs := math.Abs(impact)
	switch {
	case abs >= 2.5:
		return "extreme"
	case abs >= 1.5:
		return "strong"
	case abs >= 0.8:
		return "medium"
	default:
		return "weak"
	}
}
