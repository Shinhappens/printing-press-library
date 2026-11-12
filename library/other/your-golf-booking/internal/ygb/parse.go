// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package ygb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const pageHost = "booking.trackmangolf.com"

var slugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var nextDataRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

// ParseVenueRef accepts a venue slug or a public booking URL.
// /booking/simulators, /booking/bays, and /booking are all accepted.
// The returned bookable slug is empty when the URL does not name one.
func ParseVenueRef(raw string) (slug string, bookable string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("venue is empty")
	}
	if !strings.Contains(raw, "://") && !strings.Contains(raw, "/") {
		if !slugRE.MatchString(raw) {
			return "", "", fmt.Errorf("venue %q is not a slug or a %s URL", raw, pageHost)
		}
		return raw, "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse venue URL: %w", err)
	}
	if !strings.EqualFold(u.Hostname(), pageHost) {
		return "", "", fmt.Errorf("unsupported booking host %q; pass a %s URL or a venue slug", u.Hostname(), pageHost)
	}
	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	for i, p := range parts {
		if decoded, decErr := url.PathUnescape(p); decErr == nil {
			parts[i] = decoded
		}
	}
	if len(parts) < 2 || parts[0] != "venues" || !slugRE.MatchString(parts[1]) {
		return "", "", fmt.Errorf("could not find /venues/{slug} in %q", raw)
	}
	slug = parts[1]
	if len(parts) >= 4 && parts[2] == "booking" && slugRE.MatchString(parts[3]) {
		bookable = parts[3]
	}
	return slug, bookable, nil
}

// ParsePage reads the public Next.js payload. It keeps venue, bay, hour, and
// list-price fields and drops payment-account and external facility ids.
func ParsePage(html []byte) (Venue, error) {
	m := nextDataRE.FindSubmatch(html)
	if m == nil {
		return Venue{}, fmt.Errorf("public page has no __NEXT_DATA__ venue payload")
	}
	var envelope struct {
		Props struct {
			PageProps struct {
				InitialReduxState json.RawMessage `json:"initialReduxState"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &envelope); err != nil {
		return Venue{}, fmt.Errorf("decode page payload: %w", err)
	}
	if len(bytes.TrimSpace(envelope.Props.PageProps.InitialReduxState)) == 0 {
		return Venue{}, fmt.Errorf("public page payload has no initialReduxState")
	}
	var state rawState
	if err := json.Unmarshal(envelope.Props.PageProps.InitialReduxState, &state); err != nil {
		return Venue{}, fmt.Errorf("decode venue state: %w", err)
	}
	if !slugRE.MatchString(state.Venue.Slug) {
		return Venue{}, fmt.Errorf("public page slug %q is not usable", state.Venue.Slug)
	}
	loc, err := time.LoadLocation(state.Venue.Timezone)
	if err != nil {
		return Venue{}, fmt.Errorf("venue timezone %q: %w", state.Venue.Timezone, err)
	}
	v := Venue{
		Slug:     state.Venue.Slug,
		Name:     state.Venue.Name,
		Timezone: state.Venue.Timezone,
		Currency: strings.ToLower(state.Venue.Currency),
		Website:  state.Venue.PublicWebsite,
		Address:  formatAddress(state.Venue.Address),
		Location: loc,
	}
	for _, opt := range state.Bays.BayOptions {
		parsed, err := convertOption(opt)
		if err != nil {
			return Venue{}, err
		}
		v.Options = append(v.Options, parsed)
	}
	baysByRange := map[int][]Bay{}
	for _, bay := range state.Bays.Items {
		baysByRange[bay.Range] = append(baysByRange[bay.Range], Bay{
			ID:        bay.ID,
			Ref:       bay.Ref,
			Label:     bay.Label,
			Bookable:  bay.Bookable,
			RangeID:   bay.Range,
			OptionIDs: append([]int(nil), bay.Options...),
		})
	}
	for _, rg := range state.Ranges.Items {
		bookable := true
		if rg.Bookable != nil {
			bookable = *rg.Bookable
		}
		hours, err := ParseOpeningHours(rg.OpeningHours, rg.AssumeOpen)
		if err != nil {
			return Venue{}, fmt.Errorf("range %s: %w", rg.Slug, err)
		}
		slot := rg.SlotDuration
		if slot <= 0 {
			slot = 30
		}
		bays := baysByRange[rg.ID]
		sort.Slice(bays, func(i, j int) bool {
			if bays[i].Ref == bays[j].Ref {
				return bays[i].ID < bays[j].ID
			}
			return bays[i].Ref < bays[j].Ref
		})
		v.Ranges = append(v.Ranges, Range{
			ID:           rg.ID,
			Slug:         rg.Slug,
			Name:         rg.Name,
			SlotMinutes:  slot,
			Bookable:     bookable,
			AssumeOpen:   rg.AssumeOpen,
			OpeningHours: rg.OpeningHours,
			Hours:        hours,
			Bays:         bays,
		})
	}
	sort.Slice(v.Ranges, func(i, j int) bool { return v.Ranges[i].Slug < v.Ranges[j].Slug })
	return v, nil
}

func formatAddress(a rawAddress) string {
	parts := []string{strings.TrimSpace(a.Line1), strings.TrimSpace(a.Town), strings.TrimSpace(a.PostCode)}
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

func convertOption(opt rawOption) (BayOption, error) {
	out := BayOption{
		ID:           opt.ID,
		Name:         opt.Name,
		Group:        opt.Group,
		DisplayPrice: opt.DisplayPrice,
		PriceMinor:   opt.Price,
		Duration:     opt.Duration,
		AdminOnly:    opt.AdminOnly,
		Disabled:     opt.Disabled,
		RequiredPerk: len(opt.AppliedRequiredPerks) > 0,
	}
	if out.Duration <= 0 {
		out.Duration = 1
	}
	for _, sched := range opt.SlotSchedules {
		converted, err := convertSchedule(sched)
		if err != nil {
			return BayOption{}, fmt.Errorf("option %s: %w", opt.Name, err)
		}
		out.Schedules = append(out.Schedules, converted)
	}
	return out, nil
}

func convertSchedule(raw rawSchedule) (OptionSchedule, error) {
	out := OptionSchedule{Days: map[time.Weekday][]SchedulePoint{}}
	if raw.From != nil && strings.TrimSpace(*raw.From) != "" {
		t, err := time.Parse("2006-01-02", *raw.From)
		if err != nil {
			return OptionSchedule{}, fmt.Errorf("schedule from %q: %w", *raw.From, err)
		}
		out.From = &t
	}
	if raw.To != nil && strings.TrimSpace(*raw.To) != "" {
		t, err := time.Parse("2006-01-02", *raw.To)
		if err != nil {
			return OptionSchedule{}, fmt.Errorf("schedule to %q: %w", *raw.To, err)
		}
		out.To = &t
	}
	for dayName, points := range raw.Schedule {
		weekday, ok := weekdayToken[dayName]
		if !ok {
			return OptionSchedule{}, fmt.Errorf("schedule day %q", dayName)
		}
		var parsed []SchedulePoint
		for minuteText, value := range points {
			minute, err := strconv.Atoi(minuteText)
			if err != nil {
				return OptionSchedule{}, fmt.Errorf("schedule minute %q: %w", minuteText, err)
			}
			point := SchedulePoint{Minute: minute}
			switch typed := value.(type) {
			case nil:
			case string:
				if strings.EqualFold(typed, "unavailable") {
					point.Unavailable = true
				} else if typed != "" {
					return OptionSchedule{}, fmt.Errorf("schedule value %q", typed)
				}
			case float64:
				minor := int(typed)
				point.Price = &minor
			default:
				return OptionSchedule{}, fmt.Errorf("schedule value type %T", value)
			}
			parsed = append(parsed, point)
		}
		sort.Slice(parsed, func(i, j int) bool { return parsed[i].Minute < parsed[j].Minute })
		out.Days[weekday] = parsed
	}
	return out, nil
}

type rawState struct {
	Venue  rawVenue `json:"venue"`
	Ranges struct {
		Items []rawRange `json:"items"`
	} `json:"ranges"`
	Bays struct {
		Items      []rawBay    `json:"items"`
		BayOptions []rawOption `json:"bayOptions"`
	} `json:"bays"`
}

type rawVenue struct {
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Timezone      string     `json:"timezone"`
	Currency      string     `json:"currency"`
	PublicWebsite string     `json:"publicWebsite"`
	Address       rawAddress `json:"address"`
}

type rawAddress struct {
	Line1    string `json:"line1"`
	Town     string `json:"town"`
	PostCode string `json:"postCode"`
}

type rawRange struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	SlotDuration int    `json:"slotDuration"`
	Bookable     *bool  `json:"bookable"`
	AssumeOpen   bool   `json:"assumeOpen"`
	OpeningHours string `json:"openingHours"`
}

type rawBay struct {
	ID       int    `json:"id"`
	Ref      string `json:"ref"`
	Label    string `json:"label"`
	Bookable bool   `json:"bookable"`
	Range    int    `json:"range"`
	Options  []int  `json:"options"`
}

type rawOption struct {
	ID                   int           `json:"id"`
	Name                 string        `json:"name"`
	Group                string        `json:"group"`
	Price                int           `json:"price"`
	DisplayPrice         string        `json:"displayPrice"`
	Duration             int           `json:"duration"`
	AdminOnly            bool          `json:"adminOnly"`
	Disabled             bool          `json:"disabled"`
	AppliedRequiredPerks []int         `json:"appliedRequiredPerks"`
	SlotSchedules        []rawSchedule `json:"slotSchedules"`
}

type rawSchedule struct {
	From     *string                   `json:"from"`
	To       *string                   `json:"to"`
	Schedule map[string]map[string]any `json:"schedule"`
}
