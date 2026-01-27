package utils

import (
	"testing"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Valid formats
		{"hours and minutes", "1h 30m", 90, false},
		{"hours only", "2h", 120, false},
		{"minutes only", "45m", 45, false},
		{"plain number", "90", 90, false},
		{"decimal hours", "1.5h", 90, false},
		{"decimal hours alt", "2.5h", 150, false},
		{"zero minutes", "2h 0m", 120, false},
		{"extra whitespace", "  1h  30m  ", 90, false},
		{"no space", "1h30m", 90, false},
		{"large value", "8h 45m", 525, false},

		// Invalid formats
		{"empty string", "", 0, true},
		{"invalid text", "invalid", 0, true},
		{"negative minutes", "-30m", 0, true},
		{"zero only", "0", 0, true},
		{"zero hours zero minutes", "0h 0m", 0, true},
		{"only 'h'", "h", 0, true},
		{"only 'm'", "m", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
