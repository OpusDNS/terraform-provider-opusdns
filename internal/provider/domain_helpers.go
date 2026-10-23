package provider

import (
	"strconv"
	"strings"
)

// formatExpectedPrice renders the `expected_price` attribute as the decimal
// string the API takes (two decimal places, e.g. "1250.00"). A value with more
// than two decimal places is passed through unrounded so the API rejects it
// instead of confirming a different price than the one configured.
func formatExpectedPrice(price float64) string {
	s := strconv.FormatFloat(price, 'f', -1, 64)
	dot := strings.IndexByte(s, '.')
	switch {
	case dot < 0:
		return s + ".00"
	case len(s)-dot-1 < 2:
		return s + strings.Repeat("0", 2-(len(s)-dot-1))
	}
	return s
}
