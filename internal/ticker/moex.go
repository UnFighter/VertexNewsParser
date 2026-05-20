package ticker

const (
	BoardTQBR = "TQBR" // Основной режим торгов акциями
)

var Intervals = map[string]int{
	"1m":  1,
	"5m":  5,
	"10m": 10,
	"1h":  60,
	"1d":  24,
}
