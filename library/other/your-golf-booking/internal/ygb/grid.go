// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package ygb

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Options controls one occupancy build.
type Options struct {
	From           time.Time // inclusive civil date, midnight in the venue zone
	To             time.Time // inclusive civil date, midnight in the venue zone
	Bookables      []string
	IncludeSlots   bool
	IncludeSummary bool
	Estimate       bool
}

// Report is one venue's occupancy for the requested dates.
type Report struct {
	SourceNote string     `json:"source_note"`
	Venue      VenueView  `json:"venue"`
	From       string     `json:"from"`
	To         string     `json:"to"`
	Bookables  []string   `json:"bookables"`
	Areas      []AreaView `json:"areas"`
	Totals     Totals     `json:"totals"`
	Slots      []Slot     `json:"slots,omitempty"`
	Summary    *Summary   `json:"summary,omitempty"`
}

// VenueView is the public identity of the club. It has no payment account ids.
type VenueView struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Currency string `json:"currency"`
	Website  string `json:"website,omitempty"`
	Address  string `json:"address,omitempty"`
}

// AreaView is one bookable area included in the report.
type AreaView struct {
	Slug         string      `json:"slug"`
	Name         string      `json:"name"`
	SlotMinutes  int         `json:"slot_minutes"`
	OpeningHours string      `json:"opening_hours,omitempty"`
	Bays         []BayView   `json:"bays"`
	PublicPrices []PriceCard `json:"public_prices"`
}

// BayView is a grid column.
type BayView struct {
	Ref   string `json:"ref"`
	Label string `json:"label"`
}

// PriceCard is a guest-facing list price from the public page.
type PriceCard struct {
	Name         string `json:"name"`
	DisplayPrice string `json:"display_price,omitempty"`
	Minor        int    `json:"minor"`
	Currency     string `json:"currency"`
	Display      string `json:"display"`
	PerSlot      string `json:"per_slot"`
}

// Totals roll the slot grid up.
type Totals struct {
	OpenSlots      int     `json:"open_slots"`
	BookedSlots    int     `json:"booked_slots"`
	AvailableSlots int     `json:"available_slots"`
	OpenMinutes    int     `json:"open_minutes"`
	BookedMinutes  int     `json:"booked_minutes"`
	FillRate       float64 `json:"fill_rate"`
	Band           string  `json:"band"`
}

// Slot is one bay-time cell.
type Slot struct {
	Date              string `json:"date"`
	Start             string `json:"start"`
	End               string `json:"end"`
	Bookable          string `json:"bookable"`
	BayRef            string `json:"bay_ref"`
	BayLabel          string `json:"bay_label"`
	State             string `json:"state"`
	Status            string `json:"status,omitempty"`
	OptionName        string `json:"option_name,omitempty"`
	OptionListed      *bool  `json:"option_listed,omitempty"`
	PlayerCount       *int   `json:"player_count,omitempty"`
	GuestPriceName    string `json:"guest_price_name,omitempty"`
	GuestPriceMinor   *int   `json:"guest_price_minor,omitempty"`
	GuestPriceDisplay string `json:"guest_price_display,omitempty"`
}

// Summary is the pattern view: hour, weekday, and date.
type Summary struct {
	Note              string     `json:"note"`
	Thresholds        string     `json:"thresholds"`
	ByDate            []Bucket   `json:"by_date"`
	ByWeekday         []Bucket   `json:"by_weekday"`
	ByHour            []Bucket   `json:"by_hour"`
	Highlights        Highlights `json:"highlights"`
	ListPriceEstimate *Estimate  `json:"list_price_estimate,omitempty"`
}

// Bucket is one fill-rate row.
type Bucket struct {
	Key              string  `json:"key"`
	Hour             *int    `json:"hour,omitempty"`
	OpenMinutes      int     `json:"open_minutes"`
	BookedMinutes    int     `json:"booked_minutes"`
	AvailableMinutes int     `json:"available_minutes"`
	FillRate         float64 `json:"fill_rate"`
	Band             string  `json:"band"`
}

// Highlights names the strongest and quietest buckets. Ties keep the earliest key.
type Highlights struct {
	StrongestDate   *Bucket `json:"strongest_date,omitempty"`
	QuietestDate    *Bucket `json:"quietest_date,omitempty"`
	BusiestWeekday  *Bucket `json:"busiest_weekday,omitempty"`
	QuietestWeekday *Bucket `json:"quietest_weekday,omitempty"`
	BusiestHour     *Bucket `json:"busiest_hour,omitempty"`
	QuietestHour    *Bucket `json:"quietest_hour,omitempty"`
}

// Estimate is optional list-price math over booked slots.
type Estimate struct {
	Label                   string `json:"label"`
	Currency                string `json:"currency"`
	BookedHours             string `json:"booked_hours"`
	EstimatedMinor          int    `json:"estimated_minor"`
	EstimatedDisplay        string `json:"estimated_display"`
	KnownPriceMinutes       int    `json:"known_price_minutes"`
	ZeroListPriceMinutes    int    `json:"zero_list_price_minutes"`
	UnknownListPriceMinutes int    `json:"unknown_list_price_minutes"`
}

const thresholdNote = "quiet is fill under 0.25, moderate is 0.25 up to 0.60, busy is 0.60 or more. no_open_time means the published hours had no open bay time."

// Build turns a venue plus public bookings into an occupancy report.
func Build(v Venue, bookings []Booking, opt Options) (Report, error) {
	if v.Location == nil {
		return Report{}, fmt.Errorf("venue %s has no timezone", v.Slug)
	}
	if opt.To.Before(opt.From) {
		return Report{}, fmt.Errorf("end date is before start date")
	}
	areas, err := selectRanges(v, opt.Bookables)
	if err != nil {
		return Report{}, err
	}
	optIndex := map[int]BayOption{}
	for _, o := range v.Options {
		optIndex[o.ID] = o
	}
	byBay := map[int][]Booking{}
	for _, b := range bookings {
		if !Occupies(b.Status) || !b.End.After(b.Start) {
			continue
		}
		byBay[b.BayID] = append(byBay[b.BayID], b)
	}

	report := Report{
		SourceNote: SourceNote,
		Venue: VenueView{
			Slug: v.Slug, Name: v.Name, Timezone: v.Timezone,
			Currency: v.Currency, Website: v.Website, Address: v.Address,
		},
		From: opt.From.Format("2006-01-02"),
		To:   opt.To.Format("2006-01-02"),
	}
	var slots []builtSlot
	for _, area := range areas {
		view := AreaView{
			Slug: area.Slug, Name: area.Name, SlotMinutes: area.SlotMinutes,
			OpeningHours: area.OpeningHours,
		}
		for _, bay := range area.Bays {
			if !bay.Bookable {
				continue
			}
			view.Bays = append(view.Bays, BayView{Ref: bay.Ref, Label: bay.Label})
		}
		view.PublicPrices = publicPrices(area, v)
		report.Areas = append(report.Areas, view)
		report.Bookables = append(report.Bookables, area.Slug)

		for day := opt.From; !day.After(opt.To); day = day.AddDate(0, 0, 1) {
			intervals := area.Hours.IntervalsOn(day)
			for _, bay := range area.Bays {
				if !bay.Bookable {
					continue
				}
				for _, iv := range intervals {
					for minute := iv.Start; minute+area.SlotMinutes <= iv.End; minute += area.SlotMinutes {
						start := day.Add(time.Duration(minute) * time.Minute)
						end := start.Add(time.Duration(area.SlotMinutes) * time.Minute)
						slot := builtSlot{
							Slot: Slot{
								Date:     day.Format("2006-01-02"),
								Start:    start.Format(time.RFC3339),
								End:      end.Format(time.RFC3339),
								Bookable: area.Slug,
								BayRef:   bay.Ref,
								BayLabel: bay.Label,
								State:    "available",
							},
							start: start,
							end:   end,
						}
						if guest, minor, ok := guestPrice(area, bay, optIndex, start); ok {
							slot.GuestPriceName = guest.Name
							slot.GuestPriceMinor = &minor
							display := FormatMinor(v.Currency, minor)
							slot.GuestPriceDisplay = display
						}
						if booked, count := overlapping(byBay[bay.ID], start, end); count > 0 {
							slot.State = "booked"
							slot.Status = booked.Status
							slot.PlayerCount = booked.PlayerCount
							if optn, ok := optIndex[booked.OptionID]; ok {
								slot.OptionName = optn.Name
								listed := true
								slot.OptionListed = &listed
								slot.listMinor = listMinor(optn, start, area.SlotMinutes)
								slot.listKnown = true
							} else {
								slot.OptionName = "not on public rate card"
								listed := false
								slot.OptionListed = &listed
							}
						}
						slots = append(slots, slot)
					}
				}
			}
		}
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Start == slots[j].Start {
			if slots[i].Bookable == slots[j].Bookable {
				return slots[i].BayRef < slots[j].BayRef
			}
			return slots[i].Bookable < slots[j].Bookable
		}
		return slots[i].Start < slots[j].Start
	})

	report.Totals = totalsFrom(slots)
	if opt.IncludeSlots {
		report.Slots = make([]Slot, len(slots))
		for i := range slots {
			report.Slots[i] = slots[i].Slot
		}
	}
	if opt.IncludeSummary {
		summary := summarize(slots, opt.From, opt.To)
		if opt.Estimate {
			summary.ListPriceEstimate = estimate(v.Currency, slots)
		}
		report.Summary = &summary
	}
	return report, nil
}

type builtSlot struct {
	Slot
	start     time.Time
	end       time.Time
	listMinor int
	listKnown bool
}

func selectRanges(v Venue, want []string) ([]Range, error) {
	bySlug := map[string]Range{}
	var available []string
	for _, rg := range v.Ranges {
		if rg.Bookable {
			bySlug[rg.Slug] = rg
			available = append(available, rg.Slug)
		}
	}
	sort.Strings(available)
	if len(want) == 0 {
		var all []Range
		for _, slug := range available {
			all = append(all, bySlug[slug])
		}
		if len(all) == 0 {
			return nil, fmt.Errorf("venue %s has no bookable areas on the public page", v.Slug)
		}
		return all, nil
	}
	var out []Range
	seen := map[string]bool{}
	for _, slug := range want {
		slug = strings.TrimSpace(slug)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		rg, ok := bySlug[slug]
		if !ok {
			return nil, fmt.Errorf("bookable %q is not on the public page for %s (available: %s)", slug, v.Slug, strings.Join(available, ", "))
		}
		out = append(out, rg)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no bookable area selected")
	}
	return out, nil
}

func publicPrices(area Range, v Venue) []PriceCard {
	seen := map[int]bool{}
	var cards []PriceCard
	for _, bay := range area.Bays {
		for _, id := range bay.OptionIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			var opt *BayOption
			for i := range v.Options {
				if v.Options[i].ID == id {
					opt = &v.Options[i]
					break
				}
			}
			if opt == nil || !opt.guestFacing() {
				continue
			}
			minor := opt.slotPrice(opt.PriceMinor, area.SlotMinutes)
			cards = append(cards, PriceCard{
				Name:         opt.Name,
				DisplayPrice: strings.TrimSpace(opt.DisplayPrice),
				Minor:        minor,
				Currency:     v.Currency,
				Display:      FormatMinor(v.Currency, minor),
				PerSlot:      fmt.Sprintf("%d minutes", area.SlotMinutes),
			})
		}
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })
	return cards
}

func (o BayOption) guestFacing() bool {
	if o.AdminOnly || o.Disabled || o.RequiredPerk {
		return false
	}
	return o.PriceMinor > 0 || strings.TrimSpace(o.DisplayPrice) != ""
}

func (o BayOption) slotPrice(minor, slotMinutes int) int {
	if o.Duration <= 1 {
		return minor
	}
	return int(math.Round(float64(minor) / float64(o.Duration)))
}

func guestPrice(area Range, bay Bay, options map[int]BayOption, start time.Time) (BayOption, int, bool) {
	var best *BayOption
	bestMinor := 0
	for _, id := range bay.OptionIDs {
		opt, ok := options[id]
		if !ok || !opt.guestFacing() {
			continue
		}
		minor, available := opt.guestMinorAt(start, area.SlotMinutes)
		if !available {
			continue
		}
		if best == nil || rankGuest(opt, minor, *best, bestMinor) < 0 {
			copy := opt
			best = &copy
			bestMinor = minor
		}
	}
	if best == nil {
		return BayOption{}, 0, false
	}
	return *best, bestMinor, true
}

func rankGuest(a BayOption, aMinor int, b BayOption, bMinor int) int {
	aDisplay := strings.TrimSpace(a.DisplayPrice) != ""
	bDisplay := strings.TrimSpace(b.DisplayPrice) != ""
	if aDisplay != bDisplay {
		if aDisplay {
			return -1
		}
		return 1
	}
	if aMinor != bMinor {
		return aMinor - bMinor
	}
	return a.ID - b.ID
}

func (o BayOption) guestMinorAt(start time.Time, slotMinutes int) (int, bool) {
	state, ok := o.stateAt(start)
	if !ok {
		return o.slotPrice(o.PriceMinor, slotMinutes), true
	}
	if state.unavailable {
		return 0, false
	}
	minor := o.PriceMinor
	if state.price != nil {
		minor = *state.price
	}
	return o.slotPrice(minor, slotMinutes), true
}

func listMinor(o BayOption, start time.Time, slotMinutes int) int {
	minor := o.PriceMinor
	if state, ok := o.stateAt(start); ok && state.price != nil {
		minor = *state.price
	}
	return o.slotPrice(minor, slotMinutes)
}

type scheduleState struct {
	unavailable bool
	price       *int
}

func (o BayOption) stateAt(start time.Time) (scheduleState, bool) {
	sched := o.activeSchedule(start)
	if sched == nil {
		return scheduleState{}, false
	}
	points := sched.Days[start.Weekday()]
	if len(points) == 0 {
		return scheduleState{unavailable: true}, true
	}
	minute := start.Hour()*60 + start.Minute()
	var chosen *SchedulePoint
	for i := range points {
		if points[i].Minute <= minute {
			chosen = &points[i]
		}
	}
	if chosen == nil {
		return scheduleState{unavailable: true}, true
	}
	return scheduleState{unavailable: chosen.Unavailable, price: chosen.Price}, true
}

func (o BayOption) activeSchedule(start time.Time) *OptionSchedule {
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	var best *OptionSchedule
	bestScore := -1
	var bestFrom time.Time
	for i := range o.Schedules {
		s := &o.Schedules[i]
		if !s.covers(day) {
			continue
		}
		score := 0
		from := time.Time{}
		if s.From != nil {
			score += 2
			from = *s.From
		}
		if s.To != nil {
			score += 2
		}
		if best == nil || score > bestScore || (score == bestScore && from.After(bestFrom)) {
			best = s
			bestScore = score
			bestFrom = from
		}
	}
	return best
}

func (s OptionSchedule) covers(day time.Time) bool {
	if s.From != nil && day.Before(*s.From) {
		return false
	}
	if s.To != nil && day.After(*s.To) {
		return false
	}
	return true
}

func overlapping(bookings []Booking, start, end time.Time) (Booking, int) {
	var chosen Booking
	count := 0
	for _, b := range bookings {
		if b.Start.Before(end) && start.Before(b.End) {
			count++
			if count == 1 || b.Start.Before(chosen.Start) {
				chosen = b
			}
		}
	}
	return chosen, count
}

func totalsFrom(slots []builtSlot) Totals {
	var t Totals
	for _, s := range slots {
		mins := int(s.end.Sub(s.start).Minutes())
		t.OpenSlots++
		t.OpenMinutes += mins
		if s.State == "booked" {
			t.BookedSlots++
			t.BookedMinutes += mins
		}
	}
	t.AvailableSlots = t.OpenSlots - t.BookedSlots
	t.FillRate, t.Band = rateAndBand(t.BookedMinutes, t.OpenMinutes)
	return t
}

type minuteAcc struct {
	open, booked int
}

func summarize(slots []builtSlot, from, to time.Time) Summary {
	byDate := map[string]*minuteAcc{}
	byWeek := map[time.Weekday]*minuteAcc{}
	byHour := map[int]*minuteAcc{}
	var dates []string
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		dates = append(dates, key)
		byDate[key] = &minuteAcc{}
		byWeek[day.Weekday()] = &minuteAcc{}
	}
	for _, s := range slots {
		mins := int(s.end.Sub(s.start).Minutes())
		dateAcc := byDate[s.Date]
		if dateAcc == nil {
			dateAcc = &minuteAcc{}
			byDate[s.Date] = dateAcc
		}
		weekAcc := byWeek[s.start.Weekday()]
		if weekAcc == nil {
			weekAcc = &minuteAcc{}
			byWeek[s.start.Weekday()] = weekAcc
		}
		dateAcc.open += mins
		weekAcc.open += mins
		if s.State == "booked" {
			dateAcc.booked += mins
			weekAcc.booked += mins
		}
		cursor := s.start
		for cursor.Before(s.end) {
			nextHour := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), cursor.Hour()+1, 0, 0, 0, cursor.Location())
			if nextHour.After(s.end) {
				nextHour = s.end
			}
			part := int(nextHour.Sub(cursor).Minutes())
			hour := cursor.Hour()
			hourAcc := byHour[hour]
			if hourAcc == nil {
				hourAcc = &minuteAcc{}
				byHour[hour] = hourAcc
			}
			hourAcc.open += part
			if s.State == "booked" {
				hourAcc.booked += part
			}
			cursor = nextHour
		}
	}
	summary := Summary{Note: SourceNote, Thresholds: thresholdNote}
	for _, key := range dates {
		summary.ByDate = append(summary.ByDate, bucket(key, nil, byDate[key]))
	}
	for _, wd := range []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday} {
		if byWeek[wd] == nil {
			continue
		}
		summary.ByWeekday = append(summary.ByWeekday, bucket(wd.String(), nil, byWeek[wd]))
	}
	var hours []int
	for hour := range byHour {
		hours = append(hours, hour)
	}
	sort.Ints(hours)
	for _, hour := range hours {
		h := hour
		summary.ByHour = append(summary.ByHour, bucket(fmt.Sprintf("%02d:00", hour), &h, byHour[hour]))
	}
	summary.Highlights = Highlights{
		StrongestDate:   pick(summary.ByDate, true),
		QuietestDate:    pick(summary.ByDate, false),
		BusiestWeekday:  pick(summary.ByWeekday, true),
		QuietestWeekday: pick(summary.ByWeekday, false),
		BusiestHour:     pick(summary.ByHour, true),
		QuietestHour:    pick(summary.ByHour, false),
	}
	return summary
}

func bucket(key string, hour *int, a *minuteAcc) Bucket {
	if a == nil {
		a = &minuteAcc{}
	}
	fill, band := rateAndBand(a.booked, a.open)
	return Bucket{
		Key:              key,
		Hour:             hour,
		OpenMinutes:      a.open,
		BookedMinutes:    a.booked,
		AvailableMinutes: a.open - a.booked,
		FillRate:         fill,
		Band:             band,
	}
}

func pick(rows []Bucket, wantMax bool) *Bucket {
	var chosen *Bucket
	for i := range rows {
		if rows[i].OpenMinutes == 0 {
			continue
		}
		if chosen == nil {
			copy := rows[i]
			chosen = &copy
			continue
		}
		better := rows[i].FillRate > chosen.FillRate
		if !wantMax {
			better = rows[i].FillRate < chosen.FillRate
		}
		if better {
			copy := rows[i]
			chosen = &copy
		}
	}
	return chosen
}

func estimate(currency string, slots []builtSlot) *Estimate {
	est := &Estimate{Label: EstimateNote, Currency: currency}
	for _, s := range slots {
		if s.State != "booked" {
			continue
		}
		mins := int(s.end.Sub(s.start).Minutes())
		if !s.listKnown {
			est.UnknownListPriceMinutes += mins
			continue
		}
		est.KnownPriceMinutes += mins
		est.EstimatedMinor += s.listMinor
		if s.listMinor == 0 {
			est.ZeroListPriceMinutes += mins
		}
	}
	bookedMin := est.KnownPriceMinutes + est.UnknownListPriceMinutes
	est.BookedHours = fmt.Sprintf("%.2f", float64(bookedMin)/60)
	est.EstimatedDisplay = FormatMinor(currency, est.EstimatedMinor)
	return est
}

func rateAndBand(booked, open int) (float64, string) {
	if open <= 0 {
		return 0, "no_open_time"
	}
	rate := math.Round((float64(booked)/float64(open))*10000) / 10000
	switch {
	case rate < 0.25:
		return rate, "quiet"
	case rate < 0.60:
		return rate, "moderate"
	default:
		return rate, "busy"
	}
}

// FormatMinor renders a minor-unit amount. USD uses a dollar sign.
func FormatMinor(currency string, minor int) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	prefix := ""
	if strings.EqualFold(currency, "usd") {
		prefix = "$"
	}
	return fmt.Sprintf("%s%s%d.%02d", sign, prefix, minor/100, minor%100)
}

// CivilDay is midnight of a YYYY-MM-DD date in loc.
func CivilDay(value string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q must be YYYY-MM-DD", value)
	}
	return t, nil
}

// InclusiveDays counts civil dates from start through end, both midnights in the same zone.
func InclusiveDays(start, end time.Time) int {
	n := 0
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		n++
	}
	return n
}
