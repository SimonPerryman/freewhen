// Package availability contains pure helpers that transform external calendar
// data into the per-day availability vector the frontend renders.
package availability

import (
	"sort"
	"time"
)

// TimeRange is a half-open [Start, End) window.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// FreeDayOptions controls the "is this day free?" heuristic. Fixed for v1
// — the zero value matches the spec (≥ 2-hour block, 09:00–21:00 local).
type FreeDayOptions struct {
	// MinFreeBlock is the minimum contiguous free duration we accept. Zero
	// falls back to 2h.
	MinFreeBlock time.Duration
	// WindowStartHour (inclusive) and WindowEndHour (exclusive) frame the
	// part of the day we care about. Zero values fall back to 9 and 21.
	WindowStartHour int
	WindowEndHour   int
}

// Classified splits a date range into days that look free, busy, or
// unclassified (past days we skip). Only free/busy are meaningful for the UI —
// the caller doesn't render past days.
type Classified struct {
	Free []string // YYYY-MM-DD strings, sorted
	Busy []string
}

// ComputeFreeDays walks each day in [start, end] (inclusive) in the given
// timezone and classifies it free/busy by looking at whether any contiguous
// 2-hour gap exists in the 09:00–21:00 window. Past days (before "today" in
// tz) are filtered out. Pure — no side effects, trivially table-testable.
func ComputeFreeDays(busy []TimeRange, tz *time.Location, start, end time.Time, now time.Time, opts FreeDayOptions) Classified {
	if tz == nil {
		tz = time.UTC
	}
	if opts.MinFreeBlock == 0 {
		opts.MinFreeBlock = 2 * time.Hour
	}
	if opts.WindowStartHour == 0 && opts.WindowEndHour == 0 {
		opts.WindowStartHour = 9
		opts.WindowEndHour = 21
	}

	sorted := make([]TimeRange, 0, len(busy))
	for _, b := range busy {
		if !b.End.After(b.Start) {
			continue
		}
		sorted = append(sorted, b)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	today := time.Date(now.In(tz).Year(), now.In(tz).Month(), now.In(tz).Day(), 0, 0, 0, 0, tz)

	var out Classified
	for d := startOfDay(start, tz); !d.After(startOfDay(end, tz)); d = d.AddDate(0, 0, 1) {
		if d.Before(today) {
			continue
		}
		windowStart := time.Date(d.Year(), d.Month(), d.Day(), opts.WindowStartHour, 0, 0, 0, tz)
		windowEnd := time.Date(d.Year(), d.Month(), d.Day(), opts.WindowEndHour, 0, 0, 0, tz)
		if hasFreeBlock(sorted, windowStart, windowEnd, opts.MinFreeBlock) {
			out.Free = append(out.Free, d.Format("2006-01-02"))
		} else {
			out.Busy = append(out.Busy, d.Format("2006-01-02"))
		}
	}
	return out
}

func startOfDay(t time.Time, tz *time.Location) time.Time {
	t = t.In(tz)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
}

// hasFreeBlock reports whether [windowStart, windowEnd) has a contiguous gap
// of at least minBlock once busy ranges (sorted by Start) are subtracted.
func hasFreeBlock(busy []TimeRange, windowStart, windowEnd time.Time, minBlock time.Duration) bool {
	cursor := windowStart
	for _, b := range busy {
		if !b.End.After(windowStart) || !b.Start.Before(windowEnd) {
			continue
		}
		s := b.Start
		if s.Before(cursor) {
			s = cursor
		}
		if s.Sub(cursor) >= minBlock {
			return true
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
		if !cursor.Before(windowEnd) {
			return false
		}
	}
	return windowEnd.Sub(cursor) >= minBlock
}
