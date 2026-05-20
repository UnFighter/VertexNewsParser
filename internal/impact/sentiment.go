package impact

import (
	"math"
	"regexp"
	"strings"
	"time"
)

var positiveWords = []string{
	"рост", "прибыль", "дивиденды", "дивиденд", "рекорд", "превысил", "увеличил",
	"прирост", "улучшение", "повышение", "инвестиции", "чистая прибыль",
	"превзошёл ожидания", "позитивный", "оптимистичный",
	"growth", "profit", "dividend", "record", "revenue", "beat", "surge",
	"rise", "gain", "rally", "strong", "positive", "upgrade",
	"outperform", "bullish", "exceed", "expansion",
}

var negativeWords = []string{
	"убыток", "падение", "снижение", "кризис", "санкции", "штраф", "банкротство",
	"дефолт", "сокращение", "потери", "предупреждение", "убытки",
	"ухудшение", "риски", "понижение", "негативный", "пессимистичный",
	"loss", "decline", "crisis", "sanctions", "fine", "bankruptcy", "default",
	"cut", "warning", "miss", "fall", "drop", "weak", "negative", "downgrade",
	"underperform", "bearish", "lawsuit",
}

// числа с % или 4+ цифры — признак фактических данных
var numberRegex = regexp.MustCompile(`\d+[,.]?\d*\s*%|\d{4,}`)

var knownAuthorities = map[string]float64{
	"ria":        0.9,
	"tass":       0.9,
	"interfax":   0.9,
	"reuters":    0.95,
	"bloomberg":  0.95,
	"vedomosti":  0.85,
	"kommersant": 0.85,
	"rbc":        0.8,
	"moex":       0.95,
	"finam":      0.75,
	"bcs":        0.75,
}

// scoreText возвращает (sentiment, mentionScore, magnitude, factScore, keywordScore)
func scoreText(title, description string) (sentiment, mentionScore, magnitude, factScore, keywordScore float64) {
	text := strings.ToLower(title + " " + description)

	pos, neg := 0, 0
	for _, w := range positiveWords {
		if strings.Contains(text, w) {
			pos++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(text, w) {
			neg++
		}
	}

	factScore = scoreFactPresence(text)
	total := pos + neg
	if total == 0 {
		return 0, 0.3, 0, factScore, 0
	}

	rawScore := float64(pos-neg) / float64(total)
	sentiment = rawScore
	keywordScore = rawScore
	magnitude = math.Min(1.0, float64(total)/8.0)

	switch {
	case total >= 6:
		mentionScore = 1.5
	case total >= 4:
		mentionScore = 1.2
	case total >= 2:
		mentionScore = 0.9
	default:
		mentionScore = 0.6
	}
	return
}

func scoreFactPresence(text string) float64 {
	n := len(numberRegex.FindAllString(text, -1))
	switch {
	case n >= 3:
		return 0.9
	case n >= 1:
		return 0.6
	default:
		return 0.3
	}
}

func sourceAuthority(name string) float64 {
	lower := strings.ToLower(name)
	for key, auth := range knownAuthorities {
		if strings.Contains(lower, key) {
			return auth
		}
	}
	return 0.55
}

func noveltyScore(publishedAt time.Time) float64 {
	age := time.Since(publishedAt)
	switch {
	case age < time.Hour:
		return 1.0
	case age < 2*time.Hour:
		return 0.85
	case age < 4*time.Hour:
		return 0.7
	case age < 8*time.Hour:
		return 0.5
	case age < 24*time.Hour:
		return 0.3
	default:
		return 0.1
	}
}

// timingFactor — выше в торговые часы МБ (10:00–18:45 МСК)
func timingFactor(t time.Time) float64 {
	msk := t.UTC().Add(3 * time.Hour)
	if msk.Weekday() == time.Saturday || msk.Weekday() == time.Sunday {
		return 0.5
	}
	min := msk.Hour()*60 + msk.Minute()
	if min >= 10*60 && min <= 18*60+45 {
		return 1.3
	}
	if min >= 9*60 && min < 20*60 {
		return 0.9
	}
	return 0.6
}
