package tui

import (
	"fmt"
	"time"
)

// FormatDuration converts minutes to a human-readable string
func FormatDuration(minutes int64) string {
	if minutes == 0 {
		return "0m"
	}

	hours := minutes / 60
	mins := minutes % 60

	if hours > 0 && mins > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", mins)
}

// FormatDate formats a time.Time to a readable date string
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatDateTime formats a time.Time to a readable date and time string
func FormatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// TruncateString truncates a string to maxLen and adds "..." if needed
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatPercentage formats a float as a percentage string
func FormatPercentage(value, total float64) string {
	if total == 0 {
		return "0%"
	}
	pct := (value / total) * 100
	return fmt.Sprintf("%.1f%%", pct)
}
