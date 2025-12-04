package analyzer

import "fmt"

// FormatNumber converts a number to human-readable format
// Examples: 1800 -> "1.8k", 27022 -> "27k", 1500000 -> "1.5M"
func FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	
	if n < 10000 {
		// For numbers 1000-9999, show one decimal place if needed
		thousands := float64(n) / 1000.0
		if thousands == float64(int(thousands)) {
			return fmt.Sprintf("%.0fk", thousands)
		}
		return fmt.Sprintf("%.1fk", thousands)
	}
	
	if n < 1000000 {
		// For numbers 10000-999999, round to nearest thousand
		thousands := float64(n) / 1000.0
		if thousands == float64(int(thousands)) {
			return fmt.Sprintf("%.0fk", thousands)
		}
		// Round to one decimal place, but remove trailing zero
		rounded := float64(int(thousands*10+0.5)) / 10.0
		if rounded == float64(int(rounded)) {
			return fmt.Sprintf("%.0fk", rounded)
		}
		return fmt.Sprintf("%.1fk", rounded)
	}
	
	// For numbers >= 1,000,000, use millions
	millions := float64(n) / 1000000.0
	if millions == float64(int(millions)) {
		return fmt.Sprintf("%.0fM", millions)
	}
	return fmt.Sprintf("%.1fM", millions)
}

