// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package ygb

import "time"

// SourceNote is attached to every report. It is a limit on what the numbers mean.
const SourceNote = "Public signed-out Your Golf Booking calendar. Fill rates compare booked bay time with published opening hours. This is not official sales history: no customer identities, payments, or receipts."

// EstimateNote describes list-price math. It is not a sales total.
const EstimateNote = "Estimated by multiplying each booked slot by that option's public list price. A $0 member or perk rate is the published bay price, not membership revenue. Options missing from the public rate card are excluded. Guest add-ons, tax, and card fees are not included. This is not a sales record or a receipt."

// Venue is the public configuration needed to read occupancy.
type Venue struct {
	Slug     string
	Name     string
	Timezone string
	Currency string
	Website  string
	Address  string
	Location *time.Location
	Ranges   []Range
	Options  []BayOption
}

// Range is one bookable area (TrackMan simulators, PuttView, and so on).
type Range struct {
	ID           int
	Slug         string
	Name         string
	SlotMinutes  int
	Bookable     bool
	AssumeOpen   bool
	OpeningHours string
	Hours        Hours
	Bays         []Bay
}

// Bay is one column on the public grid.
type Bay struct {
	ID        int
	Ref       string
	Label     string
	Bookable  bool
	RangeID   int
	OptionIDs []int
}

// BayOption is a public rate-card row. PriceMinor is the list price, in minor
// currency units, for Duration slot intervals (Duration 1 is one slot).
type BayOption struct {
	ID           int
	Name         string
	Group        string
	DisplayPrice string
	PriceMinor   int
	Duration     int
	AdminOnly    bool
	Disabled     bool
	RequiredPerk bool
	Schedules    []OptionSchedule
}

// OptionSchedule is one dated price/availability overlay.
type OptionSchedule struct {
	From *time.Time
	To   *time.Time
	Days map[time.Weekday][]SchedulePoint
}

// SchedulePoint is the rate in effect from Minute (minutes after local midnight)
// until the next point. A nil Price means the option's base list price.
type SchedulePoint struct {
	Minute      int
	Unavailable bool
	Price       *int
}

// Booking is one public occupancy interval. The public payload has no customer name.
type Booking struct {
	ID          int
	Start       time.Time
	End         time.Time
	Status      string
	BayID       int
	BayRef      string
	OptionID    int
	PlayerCount *int
}

// Hours is a parsed opening-hours string.
type Hours struct {
	Weekly     map[time.Weekday][]Interval
	Exceptions []DateException
}

// Interval is [Start, End) in minutes after local midnight. End may be 1440.
type Interval struct {
	Start int
	End   int
}

// DateException replaces the weekly rule for each civil date in [From, To].
type DateException struct {
	From      time.Time
	To        time.Time
	Intervals []Interval
}

// Window is one public bookings request. End is exclusive. The API rejects
// spans longer than 24 hours.
type Window struct {
	Start time.Time
	End   time.Time
}
