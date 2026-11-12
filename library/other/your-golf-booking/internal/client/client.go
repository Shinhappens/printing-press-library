// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/internal/ygb"
)

const (
	DefaultAPIBase  = "https://api.yourgolfbooking.com"
	DefaultPageBase = "https://booking.trackmangolf.com"
	maxBody         = 8 << 20
)

// Client reads signed-out public pages and booking intervals.
type Client struct {
	APIBase   string
	PageBase  string
	HTTP      *http.Client
	UserAgent string
	limiter   *limiter
}

// New builds a client. rps is the maximum request rate; 0 disables the pause.
func New(apiBase, pageBase, userAgent string, timeout time.Duration, rps float64) *Client {
	if strings.TrimSpace(apiBase) == "" {
		apiBase = DefaultAPIBase
	}
	if strings.TrimSpace(pageBase) == "" {
		pageBase = DefaultPageBase
	}
	var interval time.Duration
	if rps > 0 {
		interval = time.Duration(float64(time.Second) / rps)
	}
	httpClient := &http.Client{Timeout: timeout}
	return &Client{
		APIBase:   strings.TrimRight(apiBase, "/"),
		PageBase:  strings.TrimRight(pageBase, "/"),
		HTTP:      httpClient,
		UserAgent: userAgent,
		limiter:   &limiter{interval: interval},
	}
}

// FetchVenue loads the public booking page. hint is the bookable slug from the
// caller's URL, when they provided one.
func (c *Client) FetchVenue(ctx context.Context, slug, hint string) (ygb.Venue, error) {
	var lastErr error
	for _, path := range pagePaths(slug, hint) {
		body, status, err := c.get(ctx, c.PageBase+path, "text/html")
		if err != nil {
			return ygb.Venue{}, err
		}
		if status == http.StatusNotFound {
			lastErr = fmt.Errorf("public page %s returned 404", path)
			continue
		}
		if status < 200 || status >= 300 {
			return ygb.Venue{}, fmt.Errorf("public page %s returned %d: %s", path, status, clipMessage(body))
		}
		venue, err := ygb.ParsePage(body)
		if err != nil {
			lastErr = fmt.Errorf("public page %s: %w", path, err)
			continue
		}
		if venue.Slug != slug {
			return ygb.Venue{}, fmt.Errorf("public page %s belongs to %s, not %s", path, venue.Slug, slug)
		}
		return venue, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no public booking page for %s", slug)
	}
	return ygb.Venue{}, lastErr
}

// FetchBookings loads public bookings whose starts fall in [start, end).
// The span is split into windows of at most 24 hours.
func (c *Client) FetchBookings(ctx context.Context, slug string, start, end time.Time) ([]ygb.Booking, error) {
	windows, err := ygb.RequestWindows(start, end)
	if err != nil {
		return nil, err
	}
	seen := map[int]struct{}{}
	var out []ygb.Booking
	for _, window := range windows {
		u := c.APIBase + "/venue/" + url.PathEscape(slug) + "/bookings/public?start_gte=" +
			url.QueryEscape(formatAPITime(window.Start)) + "&start_lte=" + url.QueryEscape(formatAPITime(window.End))
		body, status, err := c.get(ctx, u, "application/json")
		if err != nil {
			return nil, err
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("bookings for %s returned %d: %s", slug, status, clipMessage(body))
		}
		batch, err := decodeBookings(body)
		if err != nil {
			return nil, fmt.Errorf("bookings for %s: %w", slug, err)
		}
		for _, booking := range batch {
			if booking.ID != 0 {
				if _, ok := seen[booking.ID]; ok {
					continue
				}
				seen[booking.ID] = struct{}{}
			}
			out = append(out, booking)
		}
	}
	return out, nil
}

func pagePaths(slug, hint string) []string {
	prefix := "/venues/" + url.PathEscape(slug)
	var paths []string
	if hint != "" {
		paths = append(paths, prefix+"/booking/"+url.PathEscape(hint))
	}
	for _, suffix := range []string{"/booking/simulators", "/booking/bays", "/booking"} {
		path := prefix + suffix
		if len(paths) > 0 && paths[0] == path {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func (c *Client) get(ctx context.Context, rawURL, accept string) ([]byte, int, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", accept)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func formatAPITime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func decodeBookings(body []byte) ([]ygb.Booking, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	var raw []struct {
		ID            int    `json:"id"`
		Start         string `json:"start"`
		End           string `json:"end"`
		Status        string `json:"status"`
		BayID         int    `json:"bayId"`
		BayRef        string `json:"bayRef"`
		BayOptionID   int    `json:"bayOptionId"`
		PlayerOptions []struct {
			Quantity int `json:"quantity"`
		} `json:"playerOptions"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]ygb.Booking, 0, len(raw))
	for _, item := range raw {
		start, err := parseAPITime(item.Start)
		if err != nil {
			return nil, fmt.Errorf("booking start %q: %w", item.Start, err)
		}
		end, err := parseAPITime(item.End)
		if err != nil {
			return nil, fmt.Errorf("booking end %q: %w", item.End, err)
		}
		booking := ygb.Booking{
			ID: item.ID, Start: start, End: end, Status: item.Status,
			BayID: item.BayID, BayRef: item.BayRef, OptionID: item.BayOptionID,
		}
		if len(item.PlayerOptions) > 0 {
			sum := 0
			for _, player := range item.PlayerOptions {
				sum += player.Quantity
			}
			booking.PlayerCount = &sum
		}
		out = append(out, booking)
	}
	return out, nil
}

func parseAPITime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func clipMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	text := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &payload) == nil {
		if payload.Message != "" {
			text = payload.Message
		} else if payload.Error != "" {
			text = payload.Error
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 180 {
		text = text[:180]
	}
	return text
}

type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (l *limiter) Wait(ctx context.Context) error {
	if l == nil || l.interval <= 0 {
		return nil
	}
	l.mu.Lock()
	now := time.Now()
	wait := l.next.Sub(now)
	if wait < 0 {
		wait = 0
	}
	l.next = now.Add(wait + l.interval)
	l.mu.Unlock()
	if wait == 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// UserAgent identifies this read-only CLI on public requests.
func UserAgent(version string) string {
	return fmt.Sprintf("your-golf-booking-pp-cli/%s (read-only public occupancy; +https://github.com/mvanhorn/printing-press-library)", version)
}
