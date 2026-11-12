// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummaryCommand(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if strings.Contains(r.URL.Path, "/bookings/public") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":7,"start":"2026-11-13T00:00:00.000Z","end":"2026-11-13T00:30:00.000Z","status":"confirmed","bayId":10,"bayRef":"1","bayOptionId":100,"playerOptions":[]}]`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page()))
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	err := ExecuteArgs([]string{
		"summary",
		"--venue", "https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators",
		"--from", "2026-11-12",
		"--to", "2026-11-12",
		"--json",
		"--estimate-list-price",
		"--rate-limit", "0",
		"--api-base", srv.URL,
		"--page-base", srv.URL,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"band": "quiet"`, `"estimated_display": "$30.00"`, "not a sales record", "forged-golf-club", `"slug": "simulators"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if strings.Contains(out, `"slug": "bays"`) {
		t.Fatal("a /booking/simulators URL should not include the bays area")
	}
	if requests < 2 {
		t.Fatalf("expected page and booking requests, got %d", requests)
	}
}

func TestDryRunSendsNothing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := ExecuteArgs([]string{
		"occupancy",
		"--venue", "forged-golf-club",
		"--from", "2026-11-01",
		"--to", "2026-11-12",
		"--dry-run",
		"--api-base", "http://127.0.0.1:1",
		"--page-base", "http://127.0.0.1:1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"dry_run": true`) || !strings.Contains(stdout.String(), "simulators") {
		t.Fatalf("dry run: %s", stdout.String())
	}
}

func TestDoctorCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/bookings/public") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page()))
	}))
	defer srv.Close()
	var stdout, stderr bytes.Buffer
	err := ExecuteArgs([]string{
		"doctor", "--venue", "forged-golf-club", "--json", "--rate-limit", "0",
		"--api-base", srv.URL, "--page-base", srv.URL,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ok": true`) || !strings.Contains(stdout.String(), "forged-golf-club") {
		t.Fatalf("doctor: %s", stdout.String())
	}
}

func TestVersionAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := ExecuteArgs([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "your-golf-booking-pp-cli 0.1.0") {
		t.Fatalf("version: %q", stdout.String())
	}
	stdout.Reset()
	if err := ExecuteArgs([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"occupancy", "summary", "venue", "doctor", "simulators"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %s\n%s", want, stdout.String())
		}
	}
}

func page() string {
	return `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"initialReduxState":{"venue":{"slug":"forged-golf-club","name":"Forged Golf Club","timezone":"America/Chicago","currency":"usd","address":{"town":"Arlington Heights"}},"ranges":{"items":[{"id":1,"name":"Trackman Bays","slug":"simulators","slotDuration":30,"bookable":true,"assumeOpen":true,"openingHours":"Th open"},{"id":2,"name":"PuttView","slug":"bays","slotDuration":30,"bookable":true,"assumeOpen":true,"openingHours":"Th open"}]},"bays":{"items":[{"id":10,"ref":"1","label":"1","bookable":true,"range":1,"options":[100]},{"id":11,"ref":"1","label":"Puttview","bookable":true,"range":2,"options":[]}],"bayOptions":[{"id":100,"name":"Public Rate","price":3000,"displayPrice":"$60.00/Hr","duration":1,"slotSchedules":[]}]}}}}}</script>`
}
