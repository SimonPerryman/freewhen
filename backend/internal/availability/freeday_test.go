package availability

import (
	"reflect"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return loc
}

func TestComputeFreeDays(t *testing.T) {
	london := mustLoc(t, "Europe/London")
	now := time.Date(2026, 4, 23, 8, 0, 0, 0, london)

	// Two-day window: 2026-04-23 and 2026-04-24.
	start := time.Date(2026, 4, 23, 0, 0, 0, 0, london)
	end := time.Date(2026, 4, 24, 0, 0, 0, 0, london)

	cases := []struct {
		name     string
		busy     []TimeRange
		wantFree []string
		wantBusy []string
	}{
		{
			name:     "no meetings",
			busy:     nil,
			wantFree: []string{"2026-04-23", "2026-04-24"},
		},
		{
			name: "all-day block",
			busy: []TimeRange{
				{Start: time.Date(2026, 4, 23, 0, 0, 0, 0, london), End: time.Date(2026, 4, 24, 0, 0, 0, 0, london)},
			},
			wantFree: []string{"2026-04-24"},
			wantBusy: []string{"2026-04-23"},
		},
		{
			name: "30-minute meeting mid-afternoon leaves ample free time",
			busy: []TimeRange{
				{Start: time.Date(2026, 4, 23, 14, 0, 0, 0, london), End: time.Date(2026, 4, 23, 14, 30, 0, 0, london)},
			},
			wantFree: []string{"2026-04-23", "2026-04-24"},
		},
		{
			name: "back-to-back meetings leaving < 2h anywhere",
			busy: []TimeRange{
				{Start: time.Date(2026, 4, 23, 9, 0, 0, 0, london), End: time.Date(2026, 4, 23, 10, 30, 0, 0, london)},
				{Start: time.Date(2026, 4, 23, 11, 0, 0, 0, london), End: time.Date(2026, 4, 23, 12, 30, 0, 0, london)},
				{Start: time.Date(2026, 4, 23, 13, 0, 0, 0, london), End: time.Date(2026, 4, 23, 14, 30, 0, 0, london)},
				{Start: time.Date(2026, 4, 23, 15, 0, 0, 0, london), End: time.Date(2026, 4, 23, 16, 30, 0, 0, london)},
				{Start: time.Date(2026, 4, 23, 17, 0, 0, 0, london), End: time.Date(2026, 4, 23, 18, 30, 0, 0, london)},
				{Start: time.Date(2026, 4, 23, 19, 0, 0, 0, london), End: time.Date(2026, 4, 23, 20, 30, 0, 0, london)},
			},
			wantFree: []string{"2026-04-24"},
			wantBusy: []string{"2026-04-23"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeFreeDays(tc.busy, london, start, end, now, FreeDayOptions{})
			if !reflect.DeepEqual(got.Free, tc.wantFree) {
				t.Errorf("free: got %v, want %v", got.Free, tc.wantFree)
			}
			if !reflect.DeepEqual(got.Busy, tc.wantBusy) {
				t.Errorf("busy: got %v, want %v", got.Busy, tc.wantBusy)
			}
		})
	}
}

func TestComputeFreeDaysSkipsPast(t *testing.T) {
	london := mustLoc(t, "Europe/London")
	now := time.Date(2026, 4, 23, 8, 0, 0, 0, london)
	start := time.Date(2026, 4, 21, 0, 0, 0, 0, london)
	end := time.Date(2026, 4, 23, 0, 0, 0, 0, london)

	got := ComputeFreeDays(nil, london, start, end, now, FreeDayOptions{})
	want := []string{"2026-04-23"}
	if !reflect.DeepEqual(got.Free, want) {
		t.Errorf("got %v, want %v", got.Free, want)
	}
}
