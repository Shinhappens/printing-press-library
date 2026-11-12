# Your Golf Booking occupancy

Read signed-out public bay occupancy from Your Golf Booking, the calendar behind TrackMan booking pages such as `booking.trackmangolf.com`. One venue at a time. No login, checkout, cart, or account.

This is a public calendar, not a sales system. It does not return customer names, payments, or receipts.

## Install

The recommended path installs both the `your-golf-booking-pp-cli` binary and the `pp-your-golf-booking` agent skill:

```bash
npx -y @mvanhorn/printing-press install your-golf-booking
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install your-golf-booking --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available, install the CLI directly with Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/cmd/your-golf-booking-pp-cli@latest
```

This installs the CLI only.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-your-golf-booking --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-your-golf-booking --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-your-golf-booking skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-your-golf-booking. The skill defines how its required CLI can be installed.
```

## You supply the venue

There is no public directory of every `booking.trackmangolf.com` venue. The bookings API rejects `GET /venues`, the booking site has no sitemap of clubs, and this CLI does not crawl the domain. Pass a slug or a public booking URL.

These URL shapes all work:

- `https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators`
- `https://booking.trackmangolf.com/venues/forged-golf-club/booking/bays`
- `https://booking.trackmangolf.com/venues/forged-golf-club/booking`
- `forged-golf-club`

A URL that names an area (`simulators` or `bays`) reads that area. A slug with no area reads every bookable area on the page. Repeat `--venue` to query a list, one venue after another.

Requests send `User-Agent: your-golf-booking-pp-cli/<version> (read-only public occupancy; +https://github.com/mvanhorn/printing-press-library)` and wait so the default pace is one public request per second (`--rate-limit`).

## Quick start

Forged Golf Club's public simulator grid:

```bash
your-golf-booking-pp-cli occupancy \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators \
  --from 2026-11-12 --to 2026-11-12 --json
```

Fill by date, weekday, and local hour. `occupancy` defaults to the venue's local today. `summary` defaults to the seven local days ending today.

```bash
your-golf-booking-pp-cli summary \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators \
  --from 2026-11-06 --to 2026-11-12 --estimate-list-price --json
```

Check that the public page and one day of bookings respond:

```bash
your-golf-booking-pp-cli doctor \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators
```

## Commands

### venue

Public name, timezone, address as published on the page, bookable areas, bays, opening hours, and guest list prices.

```bash
your-golf-booking-pp-cli venue --venue forged-golf-club --json
```

### occupancy

Every open bay slot in the date range, marked `available` or `booked`. JSON includes the whole grid. Text output lists booked slots unless you pass `--all-slots`. A guest list price is attached when the public rate card says that cell is for sale.

The public bookings call rejects a window longer than 24 hours (`Cannot pass range greater than a day`). The CLI splits the range into shorter windows and also looks back 12 hours so a booking that started before local midnight is still visible. `--max-days` defaults to 31.

### summary

Fill rate is booked bay-minutes divided by published open bay-minutes.

| Band | Fill rate |
|---|---|
| quiet | under 0.25 |
| moderate | 0.25 up to 0.60 |
| busy | 0.60 or more |
| no_open_time | the published hours have no open bay time |

Rows are grouped by date, weekday (Monday first), and local hour. Ties keep the earliest key. Hours outside the published opening hours are omitted.

`--estimate-list-price` multiplies booked slots by the booked option's public list price. That figure is an estimate from the rate card, not a sales record, receipt, or payment total. Member and perk options often publish a $0 bay price, which is not membership revenue. Options missing from the public rate card are left out. Guest add-ons, tax, and card fees are not included.

## Past dates

The public feed accepts past windows and returns the same booking shape as future windows. A day with no bookings is an empty list and HTTP 200, not a refusal. This CLI does not know an official retention cutoff. Each run is a fresh read of whatever the feed still returns. To keep a snapshot, save the `--json` output. The CLI does not write a local database.

## Limits

- Read-only and signed out. It cannot log in, hold a bay, or pay.
- One venue at a time, from a slug or URL you already have. It will not search TrackMan or Your Golf Booking for every club.
- Occupancy is the public calendar against published opening hours. It is not official sales history.
- The address line is the street, town, and postal code fields on the public page, copied as published.
- Booking record ids, payment account ids, and external facility ids are not printed.

## Output

```bash
your-golf-booking-pp-cli summary --venue forged-golf-club --json
your-golf-booking-pp-cli summary --venue forged-golf-club --json --select totals,summary
your-golf-booking-pp-cli occupancy --venue forged-golf-club --dry-run
```

`--agent` is a shortcut for `--json`. `--select` keeps the named keys on the top-level object and on each venue.

## Agent usage

- Non-interactive: every input is a flag.
- JSON on stdout, errors on stderr.
- Read-only: no create, update, delete, book, or pay command exists.
