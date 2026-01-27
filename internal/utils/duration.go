package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseDuration converts duration strings to minutes
// Supported formats:
//   - "1h 30m" -> 90
//   - "2h" -> 120
//   - "45m" -> 45
//   - "90" -> 90 (plain number treated as minutes)
//   - "1.5h" -> 90
func ParseDuration(input string) (int64, error) {
	if input == "" {
		return 0, fmt.Errorf("duration cannot be empty")
	}

	input = strings.TrimSpace(input)

	// Try parsing as plain number (minutes)
	if num, err := strconv.ParseFloat(input, 64); err == nil {
		if num <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return int64(num), nil
	}

	// Parse hours and minutes
	var totalMinutes float64

	// Extract hours (supports decimal: 1.5h, also catches negative values)
	hoursRegex := regexp.MustCompile(`(-?\d+\.?\d*)h`)
	if matches := hoursRegex.FindStringSubmatch(input); len(matches) > 1 {
		hours, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid hours value: %v", err)
		}
		totalMinutes += hours * 60
	}

	// Extract minutes (also catches negative values)
	minutesRegex := regexp.MustCompile(`(-?\d+)m`)
	if matches := minutesRegex.FindStringSubmatch(input); len(matches) > 1 {
		minutes, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid minutes value: %v", err)
		}
		totalMinutes += minutes
	}

	if totalMinutes <= 0 {
		return 0, fmt.Errorf("invalid duration format. Use '1h 30m', '90m', or '90'")
	}

	return int64(totalMinutes), nil
}
