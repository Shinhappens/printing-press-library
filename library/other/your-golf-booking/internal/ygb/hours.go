// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package ygb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const lookback = 12 * time.Hour
const maxAPIWindow = 24 * time.Hour

var weekdayToken = map[string]time.Weekday{
	"Su": time.Sunday,
	"Mo": time.Monday,
	"Tu": time.Tuesday,
	"We": time.Wednesday,
	"Th": time.Thursday,
	"Fr": time.Friday,
	"Sa": time.Saturday,
}

var weekdayPrefix = regexp.MustCompile(`^(Su|Mo|Tu|We|Th|Fr|Sa)\s+(.+)$`)
var datePrefix = regexp.MustCompile(`^(\d{4} [A-Za-z]{3} \d{1,2})(?:\s*-\s*(\d{4} [A-Za-z]{3} \d{1,2}))?\s+(.+)$`)

// ParseOpeningHours parses Your Golf Booking opening-hours text.
// "Mo open" is the full local day. A trailing "2026 Jan 15-2026 Apr 05 off"
// closes those civil dates. An empty string with assumeOpen means every day
// is open; an empty string without it means closed.
func ParseOpeningHours(raw string, assumeOpen bool) (Hours, error) {
	h := Hours{Weekly: map[time.Weekday][]Interval{}}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if assumeOpen {
			for d := time.Sunday; d <= time.Saturday; d++ {
				h.Weekly[d] = fullDay()
			}
		}
		return h, nil
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if m := weekdayPrefix.FindStringSubmatch(part); m != nil {
			intervals, err := parseIntervals(m[2])
			if err != nil {
				return Hours{}, fmt.Errorf("opening hours %q: %w", part, err)
			}
			h.Weekly[weekdayToken[m[1]]] = intervals
			continue
		}
		if m := datePrefix.FindStringSubmatch(part); m != nil {
			from, err := parseCivil(m[1])
			if err != nil {
				return Hours{}, fmt.Errorf("opening hours %q: %w", part, err)
			}
			to := from
			if m[2] != "" {
				to, err = parseCivil(m[2])
				if err != nil {
					return Hours{}, fmt.Errorf("opening hours %q: %w", part, err)
				}
			}
			if to.Before(from) {
				return Hours{}, fmt.Errorf("opening hours %q: end date is before start date", part)
			}
			intervals, err := parseIntervals(m[3])
			if err != nil {
				return Hours{}, fmt.Errorf("opening hours %q: %w", part, err)
			}
			h.Exceptions = append(h.Exceptions, DateException{From: from, To: to, Intervals: intervals})
			continue
		}
		return Hours{}, fmt.Errorf("opening hours %q: unrecognized segment", part)
	}
	return h, nil
}

func fullDay() []Interval {
	return []Interval{{Start: 0, End: 24 * 60}}
}

func parseIntervals(spec string) ([]Interval, error) {
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "open":
		return fullDay(), nil
	case "off", "closed":
		return nil, nil
	}
	var out []Interval
	for _, piece := range strings.Split(spec, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		bounds := strings.Split(piece, "-")
		if len(bounds) != 2 {
			return nil, fmt.Errorf("expected HH:MM-HH:MM, got %q", piece)
		}
		start, err := parseClock(bounds[0])
		if err != nil {
			return nil, err
		}
		end, err := parseClock(bounds[1])
		if err != nil {
			return nil, err
		}
		if end <= start {
			return nil, fmt.Errorf("clock range %q does not move forward", piece)
		}
		out = append(out, Interval{Start: start, End: end})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no intervals in %q", spec)
	}
	return out, nil
}

func parseClock(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("clock %q is not HH:MM", raw)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("clock %q: %w", raw, err)
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("clock %q: %w", raw, err)
	}
	if hour == 24 && min == 0 {
		return 24 * 60, nil
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, fmt.Errorf("clock %q is out of range", raw)
	}
	return hour*60 + min, nil
}

func parseCivil(raw string) (time.Time, error) {
	for _, layout := range []string{"2006 Jan 2", "2006 Jan 02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("date %q is not like 2026 Jan 15", raw)
}

// IntervalsOn returns open [start, end) minute ranges for a civil date.
// Later date exceptions replace the weekly rule.
func (h Hours) IntervalsOn(day time.Time) []Interval {
	civil := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	for i := len(h.Exceptions) - 1; i >= 0; i-- {
		ex := h.Exceptions[i]
		if !civil.Before(ex.From) && !civil.After(ex.To) {
			return ex.Intervals
		}
	}
	return h.Weekly[day.Weekday()]
}

// RequestWindows splits [start, end) into spans of at most 24 hours.
// The public bookings endpoint rejects anything longer than a day.
func RequestWindows(start, end time.Time) ([]Window, error) {
	if !start.Before(end) {
		return nil, fmt.Errorf("booking window end %s is not after start %s", end.Format(time.RFC3339), start.Format(time.RFC3339))
	}
	var out []Window
	cur := start
	for cur.Before(end) {
		next := cur.Add(maxAPIWindow)
		if next.After(end) {
			next = end
		}
		if !next.After(cur) {
			break
		}
		out = append(out, Window{Start: cur, End: next})
		cur = next
	}
	return out, nil
}

// FetchSpan is the API coverage for one local civil day, including a lookback
// so a booking that started before midnight and runs into the day is visible.
// The public API filters on booking start and rejects ranges over 24 hours.
func FetchSpan(dayStart, dayEnd time.Time) (time.Time, time.Time) {
	return dayStart.Add(-lookback), dayEnd
}

// Occupies reports whether a public booking status still holds the bay.
// Unknown statuses count as occupying. Explicit cancel/void states do not.
func Occupies(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "cancelled", "canceled", "rejected", "void", "voided", "expired", "deleted", "refunded":
		return false
	default:
		return true
	}
}
