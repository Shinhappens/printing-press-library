// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/internal/ygb"
)

func TestFetchUsesShortWindowsAndUserAgent(t *testing.T) {
	var sawPage, sawBooking bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "your-golf-booking-pp-cli/0.1.0") || !strings.Contains(ua, "read-only") {
			t.Errorf("user agent %q", ua)
		}
		switch {
		case strings.Contains(r.URL.Path, "/bookings/public"):
			sawBooking = true
			start, err1 := time.Parse(time.RFC3339Nano, r.URL.Query().Get("start_gte"))
			end, err2 := time.Parse(time.RFC3339Nano, r.URL.Query().Get("start_lte"))
			if err1 != nil || err2 != nil {
				t.Errorf("times: %v %v", err1, err2)
			}
			if end.Sub(start) > 24*time.Hour {
				t.Errorf("window %s exceeds one day", end.Sub(start))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":1,"start":"2026-11-13T00:00:00.000Z","end":"2026-11-13T01:00:00.000Z","status":"attended","bayId":10,"bayRef":"1","bayOptionId":100,"playerOptions":[{"quantity":1}]}]`))
			return
		case strings.Contains(r.URL.Path, "/booking"):
			sawPage = true
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(samplePage()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, srv.URL, UserAgent("0.1.0"), 5*time.Second, 0)
	venue, err := c.FetchVenue(context.Background(), "forged-golf-club", "simulators")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	bookings, err := c.FetchBookings(context.Background(), venue.Slug, start, start.Add(30*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !sawPage || !sawBooking {
		t.Fatalf("page %v booking %v", sawPage, sawBooking)
	}
	if len(bookings) != 1 || bookings[0].PlayerCount == nil || *bookings[0].PlayerCount != 1 {
		t.Fatalf("bookings: %+v", bookings)
	}
	if bookings[0].Status != "attended" || !ygb.Occupies(bookings[0].Status) {
		t.Fatalf("status %+v", bookings[0])
	}
}

func samplePage() string {
	return `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"initialReduxState":{"venue":{"slug":"forged-golf-club","name":"Forged Golf Club","timezone":"America/Chicago","currency":"usd"},"ranges":{"items":[{"id":1,"name":"Trackman Bays","slug":"simulators","slotDuration":30,"bookable":true,"assumeOpen":true,"openingHours":"Mo open"}]},"bays":{"items":[{"id":10,"ref":"1","label":"1","bookable":true,"range":1,"options":[100]}],"bayOptions":[{"id":100,"name":"Public Rate","price":3000,"displayPrice":"$60.00/Hr","duration":1,"slotSchedules":[]}]}}}}}</script>`
}
