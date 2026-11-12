// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/internal/ygb"
	"github.com/spf13/cobra"
)

type queryFlags struct {
	venues    []string
	from      string
	to        string
	bookables []string
	maxDays   int
	allSlots  bool
	estimate  bool
}

func newVenueCmd(flags *rootFlags) *cobra.Command {
	var q queryFlags
	cmd := &cobra.Command{
		Use:   "venue",
		Short: "Show the public venue, bookable areas, bays, and list prices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReports(cmd, flags, q, runMode{kind: modeVenue})
		},
	}
	addVenueFlag(cmd, &q)
	cmd.Flags().StringArrayVar(&q.bookables, "bookable", nil, "Bookable area slug such as simulators or bays. Repeatable.")
	return cmd
}

func newOccupancyCmd(flags *rootFlags) *cobra.Command {
	var q queryFlags
	cmd := &cobra.Command{
		Use:   "occupancy",
		Short: "List available and booked slots for a date range",
		Long: `List open and booked bay slots for each civil date in the range.

Defaults to the venue's local today. Text output lists booked slots; JSON lists every open slot. Past dates are included when the public API still returns them.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReports(cmd, flags, q, runMode{kind: modeOccupancy})
		},
	}
	addQueryFlags(cmd, &q)
	cmd.Flags().BoolVar(&q.allSlots, "all-slots", false, "In text output, list available slots as well as booked slots")
	return cmd
}

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	var q queryFlags
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarize fill rate by date, weekday, and hour",
		Long: `Summarize how full the public calendar is.

Fill rate is booked bay-minutes divided by published open bay-minutes. quiet is under 0.25, moderate is under 0.60, and busy is 0.60 or more. Defaults to the seven local days ending today.

--estimate-list-price multiplies booked slots by the booked option's public list price. That number is not a sales total.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReports(cmd, flags, q, runMode{kind: modeSummary})
		},
	}
	addQueryFlags(cmd, &q)
	cmd.Flags().BoolVar(&q.estimate, "estimate-list-price", false, "Add a list-price estimate of booked slots. Not a sales total.")
	return cmd
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var q queryFlags
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that the public page and bookings feed respond",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReports(cmd, flags, q, runMode{kind: modeDoctor})
		},
	}
	addVenueFlag(cmd, &q)
	return cmd
}

func addVenueFlag(cmd *cobra.Command, q *queryFlags) {
	cmd.Flags().StringArrayVar(&q.venues, "venue", nil, "Venue slug or public booking URL. Repeat for more than one venue.")
}

func addQueryFlags(cmd *cobra.Command, q *queryFlags) {
	addVenueFlag(cmd, q)
	cmd.Flags().StringVar(&q.from, "from", "", "First civil date YYYY-MM-DD in the venue timezone")
	cmd.Flags().StringVar(&q.to, "to", "", "Last civil date YYYY-MM-DD, inclusive")
	cmd.Flags().StringArrayVar(&q.bookables, "bookable", nil, "Bookable area slug such as simulators or bays. Repeatable.")
	cmd.Flags().IntVar(&q.maxDays, "max-days", 31, "Maximum inclusive dates in one query")
}

type mode int

const (
	modeVenue mode = iota
	modeOccupancy
	modeSummary
	modeDoctor
)

type runMode struct {
	kind mode
}

func runReports(cmd *cobra.Command, flags *rootFlags, q queryFlags, mode runMode) error {
	if len(q.venues) == 0 {
		return fmt.Errorf("pass --venue with a slug or a booking.trackmangolf.com URL")
	}
	if (mode.kind == modeOccupancy || mode.kind == modeSummary) && q.maxDays <= 0 {
		return fmt.Errorf("--max-days must be at least 1")
	}
	if (q.from == "") != (q.to == "") {
		return fmt.Errorf("pass both --from and --to, or neither")
	}
	if flags.dryRun {
		return writeDryRun(cmd.OutOrStdout(), flags, q, mode)
	}
	c := client.New(flags.apiBase, flags.pageBase, client.UserAgent(version), flags.timeout, flags.rateLimit)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	switch mode.kind {
	case modeDoctor:
		return runDoctor(cmd, flags, c, q)
	case modeVenue:
		docs, err := loadVenueDocs(ctx, c, q)
		if err != nil {
			return err
		}
		return writeVenueDocs(cmd.OutOrStdout(), flags, docs)
	default:
		reports, err := loadOccupancy(ctx, c, q, mode)
		if err != nil {
			return err
		}
		return writeEnvelope(cmd.OutOrStdout(), flags, reports, mode.kind == modeOccupancy, q.allSlots)
	}
}

type venueDoc struct {
	Venue ygb.VenueView  `json:"venue"`
	Areas []ygb.AreaView `json:"areas"`
}

func loadVenueDocs(ctx context.Context, c *client.Client, q queryFlags) ([]venueDoc, error) {
	var docs []venueDoc
	for _, input := range q.venues {
		venue, hint, err := loadVenue(ctx, c, input)
		if err != nil {
			return nil, err
		}
		bookables := q.bookables
		if len(bookables) == 0 && hint != "" {
			bookables = []string{hint}
		}
		today := time.Now().In(venue.Location)
		day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, venue.Location)
		report, err := ygb.Build(venue, nil, ygb.Options{From: day, To: day, Bookables: bookables})
		if err != nil {
			return nil, err
		}
		docs = append(docs, venueDoc{Venue: report.Venue, Areas: report.Areas})
	}
	return docs, nil
}

func loadOccupancy(ctx context.Context, c *client.Client, q queryFlags, mode runMode) ([]ygb.Report, error) {
	var reports []ygb.Report
	for _, input := range q.venues {
		venue, hint, err := loadVenue(ctx, c, input)
		if err != nil {
			return nil, err
		}
		from, to, err := resolveDates(q, venue.Location, mode.kind)
		if err != nil {
			return nil, err
		}
		if days := ygb.InclusiveDays(from, to); days > q.maxDays {
			return nil, fmt.Errorf("%s covers %d dates; raise --max-days (currently %d) to continue", venue.Slug, days, q.maxDays)
		}
		bookables := q.bookables
		if len(bookables) == 0 && hint != "" {
			bookables = []string{hint}
		}
		spanStart, spanEnd := ygb.FetchSpan(from, to.AddDate(0, 0, 1))
		bookings, err := c.FetchBookings(ctx, venue.Slug, spanStart, spanEnd)
		if err != nil {
			return nil, err
		}
		report, err := ygb.Build(venue, bookings, ygb.Options{
			From:           from,
			To:             to,
			Bookables:      bookables,
			IncludeSlots:   mode.kind == modeOccupancy,
			IncludeSummary: mode.kind == modeSummary,
			Estimate:       mode.kind == modeSummary && q.estimate,
		})
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func loadVenue(ctx context.Context, c *client.Client, input string) (ygb.Venue, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	slug, hint, err := ygb.ParseVenueRef(input)
	if err != nil {
		return ygb.Venue{}, "", err
	}
	venue, err := c.FetchVenue(ctx, slug, hint)
	if err != nil {
		return ygb.Venue{}, "", err
	}
	return venue, hint, nil
}

func resolveDates(q queryFlags, loc *time.Location, kind mode) (time.Time, time.Time, error) {
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if q.from == "" && q.to == "" {
		if kind == modeSummary {
			return today.AddDate(0, 0, -6), today, nil
		}
		return today, today, nil
	}
	from, err := ygb.CivilDay(q.from, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := ygb.CivilDay(q.to, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to is before --from")
	}
	return from, to, nil
}

func runDoctor(cmd *cobra.Command, flags *rootFlags, c *client.Client, q queryFlags) error {
	type check struct {
		OK             bool     `json:"ok"`
		Venue          string   `json:"venue"`
		Name           string   `json:"name"`
		Timezone       string   `json:"timezone"`
		Bookables      []string `json:"bookables"`
		SampleDay      string   `json:"sample_day"`
		PublicBookings int      `json:"public_bookings"`
		UserAgent      string   `json:"user_agent"`
	}
	var checks []check
	for _, input := range q.venues {
		venue, _, err := loadVenue(cmd.Context(), c, input)
		if err != nil {
			return err
		}
		today := time.Now().In(venue.Location)
		day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, venue.Location)
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		bookings, err := c.FetchBookings(ctx, venue.Slug, day, day.AddDate(0, 0, 1))
		if err != nil {
			return err
		}
		var slugs []string
		for _, area := range venue.Ranges {
			if area.Bookable {
				slugs = append(slugs, area.Slug)
			}
		}
		checks = append(checks, check{
			OK: true, Venue: venue.Slug, Name: venue.Name, Timezone: venue.Timezone,
			Bookables: slugs, SampleDay: day.Format("2006-01-02"),
			PublicBookings: len(bookings), UserAgent: c.UserAgent,
		})
	}
	payload := map[string]any{"ok": true, "checks": checks}
	if !flags.asJSON && strings.TrimSpace(flags.selectFields) == "" {
		for _, item := range checks {
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) %s bookables=%s sample_day=%s public_bookings=%d\n",
				item.Name, item.Venue, item.Timezone, strings.Join(item.Bookables, ","), item.SampleDay, item.PublicBookings)
		}
		return nil
	}
	return writeJSON(cmd.OutOrStdout(), payload, flags.selectFields)
}

func writeVenueDocs(w io.Writer, flags *rootFlags, docs []venueDoc) error {
	payload := map[string]any{
		"source_note": ygb.SourceNote,
		"queried_at":  time.Now().UTC().Format(time.RFC3339),
		"venues":      docs,
	}
	if flags.asJSON || strings.TrimSpace(flags.selectFields) != "" {
		return writeJSON(w, payload, flags.selectFields)
	}
	for _, doc := range docs {
		fmt.Fprintf(w, "%s (%s) timezone %s\n", doc.Venue.Name, doc.Venue.Slug, doc.Venue.Timezone)
		if doc.Venue.Address != "" {
			fmt.Fprintf(w, "address %s\n", doc.Venue.Address)
		}
		for _, area := range doc.Areas {
			fmt.Fprintf(w, "bookable %s (%s), %d bays, %d-minute slots, hours %s\n",
				area.Slug, area.Name, len(area.Bays), area.SlotMinutes, area.OpeningHours)
			for _, price := range area.PublicPrices {
				fmt.Fprintf(w, "  public price %s %s\n", price.Name, price.Display)
				if price.DisplayPrice != "" {
					fmt.Fprintf(w, "  rate card %s\n", price.DisplayPrice)
				}
			}
		}
	}
	fmt.Fprintln(w, ygb.SourceNote)
	return nil
}

func writeEnvelope(w io.Writer, flags *rootFlags, reports []ygb.Report, textIsOccupancy bool, allSlots bool) error {
	payload := map[string]any{
		"source_note": ygb.SourceNote,
		"queried_at":  time.Now().UTC().Format(time.RFC3339),
		"venues":      reports,
	}
	if flags.asJSON || strings.TrimSpace(flags.selectFields) != "" {
		return writeJSON(w, payload, flags.selectFields)
	}
	for _, report := range reports {
		if err := writeText(w, report, textIsOccupancy, allSlots); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, payload any, selectFields string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	if fields := splitSelect(selectFields); len(fields) > 0 {
		decoded = filterJSON(decoded, fields)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(decoded)
}

func splitSelect(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func filterJSON(value any, keys map[string]bool) any {
	obj, ok := value.(map[string]any)
	if !ok {
		return value
	}
	out := map[string]any{}
	for key, child := range obj {
		if keys[key] {
			out[key] = child
		}
	}
	venues, hasVenues := obj["venues"].([]any)
	if !hasVenues {
		return out
	}
	filtered := make([]any, 0, len(venues))
	for _, item := range venues {
		venue, ok := item.(map[string]any)
		if !ok {
			continue
		}
		child := map[string]any{}
		for key, val := range venue {
			if keys[key] {
				child[key] = val
			}
		}
		filtered = append(filtered, child)
	}
	if len(filtered) > 0 && !keys["venues"] {
		out["venues"] = filtered
	}
	return out
}

func writeText(w io.Writer, report ygb.Report, occupancy bool, allSlots bool) error {
	fmt.Fprintf(w, "%s (%s) %s to %s\n", report.Venue.Name, report.Venue.Slug, report.From, report.To)
	fmt.Fprintf(w, "timezone %s  fill %.4f %s  booked %d/%d slots\n",
		report.Venue.Timezone, report.Totals.FillRate, report.Totals.Band, report.Totals.BookedSlots, report.Totals.OpenSlots)
	for _, area := range report.Areas {
		prices := make([]string, 0, len(area.PublicPrices))
		for _, price := range area.PublicPrices {
			label := price.Display
			if price.DisplayPrice != "" {
				label = price.DisplayPrice + " (" + price.Display + "/" + strings.TrimSuffix(price.PerSlot, " minutes") + "min)"
			}
			prices = append(prices, price.Name+" "+label)
		}
		fmt.Fprintf(w, "bookable %s (%s), %d bays, %d-minute slots", area.Slug, area.Name, len(area.Bays), area.SlotMinutes)
		if len(prices) > 0 {
			fmt.Fprintf(w, "; public price %s", strings.Join(prices, ", "))
		}
		fmt.Fprintln(w)
	}
	if occupancy {
		shown := 0
		for _, slot := range report.Slots {
			if slot.State != "booked" && !allSlots {
				continue
			}
			shown++
			price := ""
			if slot.GuestPriceDisplay != "" {
				price = " guest_price " + slot.GuestPriceDisplay
			}
			fmt.Fprintf(w, "%s %s-%s bay %s %s %s%s\n",
				slot.Date, clock(slot.Start), clock(slot.End), slot.BayLabel, slot.State, slot.OptionName, price)
		}
		if shown == 0 {
			fmt.Fprintln(w, "no booked slots in the published open hours")
		}
	}
	if report.Summary != nil {
		fmt.Fprintln(w, "by date")
		for _, row := range report.Summary.ByDate {
			fmt.Fprintf(w, "  %s  fill %.4f  %s  booked %d min / open %d min\n", row.Key, row.FillRate, row.Band, row.BookedMinutes, row.OpenMinutes)
		}
		fmt.Fprintln(w, "by weekday")
		for _, row := range report.Summary.ByWeekday {
			fmt.Fprintf(w, "  %s  fill %.4f  %s  booked %d min / open %d min\n", row.Key, row.FillRate, row.Band, row.BookedMinutes, row.OpenMinutes)
		}
		fmt.Fprintln(w, "by hour (venue local)")
		for _, row := range report.Summary.ByHour {
			fmt.Fprintf(w, "  %s  fill %.4f  %s  booked %d min / open %d min\n", row.Key, row.FillRate, row.Band, row.BookedMinutes, row.OpenMinutes)
		}
		if h := report.Summary.Highlights.StrongestDate; h != nil {
			fmt.Fprintf(w, "strongest date %s fill %.4f %s\n", h.Key, h.FillRate, h.Band)
		}
		if h := report.Summary.Highlights.QuietestDate; h != nil {
			fmt.Fprintf(w, "quietest date %s fill %.4f %s\n", h.Key, h.FillRate, h.Band)
		}
		if h := report.Summary.Highlights.BusiestHour; h != nil {
			fmt.Fprintf(w, "busiest hour %s fill %.4f %s\n", h.Key, h.FillRate, h.Band)
		}
		if h := report.Summary.Highlights.QuietestHour; h != nil {
			fmt.Fprintf(w, "quietest hour %s fill %.4f %s\n", h.Key, h.FillRate, h.Band)
		}
		if est := report.Summary.ListPriceEstimate; est != nil {
			fmt.Fprintf(w, "list-price estimate %s over %s booked hours (%d minutes at a $0 list price, %d minutes with no public list price)\n",
				est.EstimatedDisplay, est.BookedHours, est.ZeroListPriceMinutes, est.UnknownListPriceMinutes)
			fmt.Fprintln(w, est.Label)
		}
	}
	fmt.Fprintln(w, report.SourceNote)
	return nil
}

func clock(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Format("15:04")
}

func writeDryRun(w io.Writer, flags *rootFlags, q queryFlags, mode runMode) error {
	type venuePlan struct {
		Input             string `json:"input"`
		Slug              string `json:"slug"`
		BookableFromURL   string `json:"bookable_from_url,omitempty"`
		PageURL           string `json:"page_url"`
		From              string `json:"from,omitempty"`
		To                string `json:"to,omitempty"`
		BookingURLPattern string `json:"booking_url_pattern"`
	}
	plans := make([]venuePlan, 0, len(q.venues))
	for _, input := range q.venues {
		slug, hint, err := ygb.ParseVenueRef(input)
		if err != nil {
			return err
		}
		if q.from != "" {
			if _, err := ygb.CivilDay(q.from, time.UTC); err != nil {
				return err
			}
			if _, err := ygb.CivilDay(q.to, time.UTC); err != nil {
				return err
			}
			from, _ := ygb.CivilDay(q.from, time.UTC)
			to, _ := ygb.CivilDay(q.to, time.UTC)
			if days := ygb.InclusiveDays(from, to); days > q.maxDays && mode.kind != modeVenue && mode.kind != modeDoctor {
				return fmt.Errorf("%s covers %d dates; raise --max-days (currently %d) to continue", slug, days, q.maxDays)
			}
		}
		page := strings.TrimRight(flags.pageBase, "/") + "/venues/" + url.PathEscape(slug) + "/booking"
		if hint != "" {
			page += "/" + url.PathEscape(hint)
		} else {
			page += "/simulators"
		}
		plans = append(plans, venuePlan{
			Input: input, Slug: slug, BookableFromURL: hint, PageURL: page,
			From: q.from, To: q.to,
			BookingURLPattern: strings.TrimRight(flags.apiBase, "/") + "/venue/" + url.PathEscape(slug) + "/bookings/public?start_gte=START&start_lte=END",
		})
	}
	note := "No request was sent. Live booking calls use the venue timezone, a 12 hour lookback, and windows of at most 24 hours because the public API rejects longer ranges. Occupancy defaults to the local today. Summary defaults to the seven local days ending today. Venues are queried one after another."
	if mode.kind == modeVenue || mode.kind == modeDoctor {
		note = "No request was sent. A live call would read the public booking page and, for doctor, one local day of public bookings."
	}
	return writeJSON(w, map[string]any{
		"dry_run":    true,
		"note":       note,
		"user_agent": client.UserAgent(version),
		"venues":     plans,
	}, flags.selectFields)
}
