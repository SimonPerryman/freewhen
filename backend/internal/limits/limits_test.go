package limits

import (
	"testing"
	"time"
)

func TestDateWithinRange(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		date time.Time
		want bool
	}{
		{"today", now, true},
		{"one day past min", now.AddDate(-1, 0, 1), true},
		{"exactly min", now.AddDate(-1, 0, 0), true},
		{"one day before min", now.AddDate(-1, 0, -1), false},
		{"far past", time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC), false},
		{"exactly max", now.AddDate(5, 0, 0), true},
		{"one day before max", now.AddDate(5, 0, -1), true},
		{"one day past max", now.AddDate(5, 0, 1), false},
		{"year 9999", time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DateWithinRange(tc.date, now); got != tc.want {
				t.Fatalf("DateWithinRange(%s) = %v, want %v", tc.date, got, tc.want)
			}
		})
	}
}
