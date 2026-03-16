package model

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		minutes int
		wantErr bool
	}{
		{"30m", 30, false},
		{"2h", 120, false},
		{"1.5h", 90, false},
		{"1d", 480, false},
		{"1.5d", 720, false},
		{"1w", 2400, false},
		{"0.5w", 1200, false},
		{"12h", 720, false},
		{"4h", 240, false},
		{"-2h", -120, false},

		// Errors
		{"", 0, true},
		{"2x", 0, true},
		{"abc", 0, true},
		{"h", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDuration(%q) expected error, got %+v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDuration(%q) unexpected error: %v", tt.input, err)
			}
			if got.Minutes != tt.minutes {
				t.Errorf("ParseDuration(%q).Minutes = %d, want %d", tt.input, got.Minutes, tt.minutes)
			}
			if got.Raw != tt.input {
				t.Errorf("ParseDuration(%q).Raw = %q, want %q", tt.input, got.Raw, tt.input)
			}
		})
	}
}

func TestDuration_TimeDuration(t *testing.T) {
	d := Duration{Minutes: 120}
	want := 2 * time.Hour
	if got := d.TimeDuration(); got != want {
		t.Errorf("Duration{120}.TimeDuration() = %v, want %v", got, want)
	}
}

func TestDuration_String(t *testing.T) {
	d := Duration{Minutes: 120, Raw: "2h"}
	if got := d.String(); got != "2h" {
		t.Errorf("got %q, want %q", got, "2h")
	}

	d2 := Duration{Minutes: 90}
	if got := d2.String(); got != "90m" {
		t.Errorf("got %q, want %q", got, "90m")
	}
}

func TestDuration_IsZero(t *testing.T) {
	if !(Duration{}).IsZero() {
		t.Error("zero duration should be zero")
	}
	if (Duration{Minutes: 1}).IsZero() {
		t.Error("non-zero duration should not be zero")
	}
}
