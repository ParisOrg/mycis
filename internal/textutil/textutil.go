package textutil

import (
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

func TrimPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func ParseDateOnly(value string) (time.Time, error) {
	return time.Parse(DateLayout, value)
}
