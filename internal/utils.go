package internal

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

var sanitizer = bluemonday.StrictPolicy()

type Source struct {
	ID   int
	Name string
	URL  string
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func hashTitle(title string) string {
	h := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(title))))
	return hex.EncodeToString(h[:])
}

func sanitizeAndTruncate(s string, maxLen int) string {
	s = sanitizer.Sanitize(strings.TrimSpace(s))
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}
