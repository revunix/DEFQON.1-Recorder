package util

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:          "0 Bytes",
		1024:       "1.00 KB",
		1536:       "1.50 KB",
		1048576:    "1.00 MB",
		1073741824: "1.00 GB",
		1610612736: "1.50 GB",
	}
	for in, want := range cases {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[int]string{
		0:    "Live",
		30:   "30m",
		60:   "1h",
		90:   "1h 30m",
		1440: "1d",
		1500: "1d 1h",
	}
	for in, want := range cases {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatThousands(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		999:     "999",
		1000:    "1.000",
		1234567: "1.234.567",
	}
	for in, want := range cases {
		if got := FormatThousands(in); got != want {
			t.Errorf("FormatThousands(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestBerlinUsesSummerTimeInJune(t *testing.T) {
	loc := Berlin()
	start := time.Date(2026, time.June, 25, 20, 0, 0, 0, loc)
	_, offset := start.Zone()
	if offset != 2*60*60 {
		t.Fatalf("2026-06-25 20:00 zone offset = %d seconds, want %d", offset, 2*60*60)
	}
	if got := start.UTC().Format("15:04"); got != "18:00" {
		t.Fatalf("2026-06-25 20:00 event time resolves to UTC %s, want 18:00", got)
	}
}

func TestSanitize(t *testing.T) {
	if got := Sanitize("ABC abc 123 -_"); got != "ABC abc 123 -_" {
		t.Errorf("Sanitize changed clean string: %q", got)
	}
	if got := Sanitize("üïöé"); got != "" {
		t.Errorf("expected non-ASCII stripped, got %q", got)
	}
}
