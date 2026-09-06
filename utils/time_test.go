package utils

import (
	"testing"
	"time"
)

func TestFormatTimeSince(t *testing.T) {
	now := time.Now()
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{5 * time.Minute, "5 minutes ago"},
		{time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{3 * 24 * time.Hour, "3 days ago"},
		{2 * 7 * 24 * time.Hour, "2 weeks ago"},
		{31 * 24 * time.Hour, "1 month ago"},
		{2 * 365 * 24 * time.Hour, "2 years ago"},
	}
	for _, tc := range cases {
		if got := FormatTimeSince(now.Add(-tc.ago)); got != tc.want {
			t.Errorf("FormatTimeSince(-%v) = %q, want %q", tc.ago, got, tc.want)
		}
	}
}

func TestFormatTimeShort(t *testing.T) {
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-10 * time.Second), "now"},
		{now.Add(-5 * time.Minute), "5m"},
		{now.Add(-3 * time.Hour), "3h"},
		{now.Add(-2 * 24 * time.Hour), "2d"},
		{time.Date(2026, time.January, 2, 8, 0, 0, 0, time.UTC), "Jan 2"},
		{time.Date(2024, time.June, 1, 8, 0, 0, 0, time.UTC), "2024"},
	}
	for _, tc := range cases {
		if got := FormatTimeShort(tc.t, now); got != tc.want {
			t.Errorf("FormatTimeShort(%v) = %q, want %q", tc.t, got, tc.want)
		}
		if len([]rune(FormatTimeShort(tc.t, now))) > 5 {
			t.Errorf("FormatTimeShort(%v) wider than 5 cells", tc.t)
		}
	}
}
