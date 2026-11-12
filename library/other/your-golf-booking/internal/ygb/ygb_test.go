// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package ygb

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseVenueRef(t *testing.T) {
	cases := []struct {
		in, slug, bookable string
	}{
		{"forged-golf-club", "forged-golf-club", ""},
		{"https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators", "forged-golf-club", "simulators"},
		{"https://booking.trackmangolf.com/venues/forged-golf-club/booking/bays", "forged-golf-club", "bays"},
		{"https://booking.trackmangolf.com/venues/forged-golf-club/booking", "forged-golf-club", ""},
		{"https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators?date=2026-11-12", "forged-golf-club", "simulators"},
		{"booking.trackmangolf.com/venues/forged-golf-club/booking/simulators/", "forged-golf-club", "simulators"},
	}
	for _, tc := range cases {
		slug, bookable, err := ParseVenueRef(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if slug != tc.slug || bookable != tc.bookable {
			t.Fatalf("%s: got %s %s", tc.in, slug, bookable)
		}
	}
	if _, _, err := ParseVenueRef("https://example.com/venues/forged-golf-club/booking"); err == nil {
		t.Fatal("expected unsupported host to fail")
	}
}

func TestOpeningHoursAndWindows(t *testing.T) {
	hours, err := ParseOpeningHours("Mo open;Tu open;We open;Th open;Fr open;Sa open;Su open;2026 Jan 15-2026 Apr 05 off", true)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	thu := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	if got := hours.IntervalsOn(thu); len(got) != 1 || got[0].End != 24*60 {
		t.Fatalf("thursday intervals: %+v", got)
	}
	closed := time.Date(2026, 1, 20, 0, 0, 0, 0, loc)
	if got := hours.IntervalsOn(closed); len(got) != 0 {
		t.Fatalf("closed date intervals: %+v", got)
	}
	start := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	end := start.Add(40 * time.Hour)
	windows, err := RequestWindows(start, end)
	if err != nil {
		t.Fatal(err)
	}
	covered := start
	for _, w := range windows {
		if w.End.Sub(w.Start) > 24*time.Hour {
			t.Fatalf("window longer than a day: %s", w.End.Sub(w.Start))
		}
		if !w.Start.Equal(covered) {
			t.Fatalf("gap before %s", w.Start)
		}
		covered = w.End
	}
	if !covered.Equal(end) {
		t.Fatalf("coverage ended at %s", covered)
	}
}

func TestOccupancySummaryAndEstimate(t *testing.T) {
	venue, err := ParsePage([]byte(fixturePage()))
	if err != nil {
		t.Fatal(err)
	}
	if venue.Name != "Forged Golf Club" || venue.Timezone != "America/Chicago" {
		t.Fatalf("venue: %+v", venue)
	}
	loc := venue.Location
	day := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	players := 2
	bookings := []Booking{
		{
			ID: 1, Start: day.Add(18 * time.Hour), End: day.Add(19 * time.Hour),
			Status: "attended", BayID: 10, OptionID: 100, PlayerCount: &players,
		},
		{
			ID: 2, Start: day.Add(18 * time.Hour), End: day.Add(18*time.Hour + 30*time.Minute),
			Status: "confirmed", BayID: 11, OptionID: 999,
		},
		{
			ID: 3, Start: day.Add(10 * time.Hour), End: day.Add(11 * time.Hour),
			Status: "cancelled", BayID: 10, OptionID: 100,
		},
	}
	report, err := Build(venue, bookings, Options{
		From: day, To: day, Bookables: []string{"simulators"},
		IncludeSlots: true, IncludeSummary: true, Estimate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.OpenSlots != 96 || report.Totals.BookedSlots != 3 {
		t.Fatalf("totals: %+v", report.Totals)
	}
	if report.Totals.Band != "quiet" {
		t.Fatalf("band %s fill %v", report.Totals.Band, report.Totals.FillRate)
	}
	var guestBooked int
	for _, slot := range report.Slots {
		if slot.State == "booked" && slot.OptionName == "Public Rate" {
			guestBooked++
			if slot.GuestPriceDisplay != "$30.00" || slot.PlayerCount == nil || *slot.PlayerCount != 2 {
				t.Fatalf("public slot: %+v", slot)
			}
		}
		if slot.State == "booked" && strings.Contains(slot.OptionName, "not on public") && slot.GuestPriceDisplay != "$30.00" {
			t.Fatalf("unknown option slot should still show the guest rate: %+v", slot)
		}
	}
	if guestBooked != 2 {
		t.Fatalf("public booked slots %d", guestBooked)
	}
	var hour18 *Bucket
	for i := range report.Summary.ByHour {
		if report.Summary.ByHour[i].Key == "18:00" {
			hour18 = &report.Summary.ByHour[i]
		}
	}
	if hour18 == nil || hour18.BookedMinutes != 90 || hour18.OpenMinutes != 120 || hour18.Band != "busy" {
		t.Fatalf("hour 18: %+v", hour18)
	}
	if report.Summary.Highlights.BusiestHour == nil || report.Summary.Highlights.BusiestHour.Key != "18:00" {
		t.Fatalf("highlights: %+v", report.Summary.Highlights)
	}
	est := report.Summary.ListPriceEstimate
	if est == nil || est.EstimatedMinor != 6000 || est.EstimatedDisplay != "$60.00" || est.UnknownListPriceMinutes != 30 {
		t.Fatalf("estimate: %+v", est)
	}
	if !strings.Contains(est.Label, "not a sales record") || !strings.Contains(report.SourceNote, "not official sales history") {
		t.Fatal("missing estimate or source label")
	}
	closedDay := time.Date(2026, 1, 20, 0, 0, 0, 0, loc)
	closed, err := Build(venue, nil, Options{From: closedDay, To: closedDay, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if closed.Totals.Band != "no_open_time" || closed.Totals.OpenSlots != 0 {
		t.Fatalf("closed day: %+v", closed.Totals)
	}
}

func fixturePage() string {
	state := map[string]any{
		"venue": map[string]any{
			"slug": "forged-golf-club", "name": "Forged Golf Club",
			"timezone": "America/Chicago", "currency": "usd",
			"publicWebsite": "forgedgolfclub.com",
			"address":       map[string]any{"line1": "650 E Kensington Rd", "town": "Arlington Heights", "postCode": "60004"},
		},
		"ranges": map[string]any{"items": []any{map[string]any{
			"id": 1, "name": "Trackman Bays", "slug": "simulators", "slotDuration": 30,
			"bookable": true, "assumeOpen": true,
			"openingHours": "Mo open;Tu open;We open;Th open;Fr open;Sa open;Su open;2026 Jan 15-2026 Apr 05 off",
		}}},
		"bays": map[string]any{
			"items": []any{
				map[string]any{"id": 10, "ref": "1", "label": "1", "bookable": true, "range": 1, "options": []any{100, 200}},
				map[string]any{"id": 11, "ref": "2", "label": "2", "bookable": true, "range": 1, "options": []any{100}},
			},
			"bayOptions": []any{
				map[string]any{
					"id": 100, "name": "Public Rate", "group": "Public", "price": 3000,
					"displayPrice": "$60.00/Hr", "duration": 1,
					"adminOnly": false, "disabled": false, "appliedRequiredPerks": []any{},
					"slotSchedules": []any{map[string]any{
						"from": nil, "to": nil,
						"schedule": allDaySchedule(nil),
					}},
				},
				map[string]any{
					"id": 200, "name": "Annual Member Rate", "price": 0, "displayPrice": " ",
					"duration": 1, "appliedRequiredPerks": []any{1}, "slotSchedules": []any{},
				},
			},
		},
	}
	body, err := json.Marshal(map[string]any{
		"props": map[string]any{"pageProps": map[string]any{"initialReduxState": state}},
	})
	if err != nil {
		panic(err)
	}
	return `<script id="__NEXT_DATA__" type="application/json">` + string(body) + `</script>`
}

func allDaySchedule(value any) map[string]any {
	day := map[string]any{"0": value}
	return map[string]any{"Mo": day, "Tu": day, "We": day, "Th": day, "Fr": day, "Sa": day, "Su": day}
}
