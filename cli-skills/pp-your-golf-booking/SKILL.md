---
name: pp-your-golf-booking
description: "Read public Your Golf Booking and TrackMan bay occupancy for one venue. Trigger phrases: `golf bay occupancy`, `TrackMan booking calendar`, `Your Golf Booking slots`, `how full is the simulator`, `forged golf club bookings`, `use your-golf-booking`."
author: "Printing Press Library"
license: "Apache-2.0"
argument-hint: "<command> [flags]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - your-golf-booking-pp-cli
    install:
      - kind: go
        bins: [your-golf-booking-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/cmd/your-golf-booking-pp-cli
---

# Your Golf Booking — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `your-golf-booking-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install your-golf-booking --cli-only
   ```
2. Verify: `your-golf-booking-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/your-golf-booking/cmd/your-golf-booking-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI to read public simulator or bay occupancy for a Your Golf Booking venue you already know, including TrackMan booking pages on `booking.trackmangolf.com`. It answers how full a venue is over a date range, which hours and weekdays look busy or quiet, and what guest list price the public page shows.

The caller must pass a venue slug or a public URL. These all work: `/booking/simulators`, `/booking/bays`, and `/booking`.

## When Not to Use This CLI

Do not use this CLI to log in, book a bay, check out, pay, or read customer names, payments, or receipts. The public feed does not include them.

Do not use it to discover every TrackMan or Your Golf Booking venue. There is no public all-venues index in this tool, and it does not crawl `booking.trackmangolf.com`.

## Limits

- One venue per call. Repeat `--venue` to read a list, one after another, at the `--rate-limit` pace (default one request per second).
- Past dates work when the public feed still has them. An empty day is an empty list, not proof the API rejected history. Save `--json` output if you need a snapshot later. The CLI does not store one.
- The bookings API rejects ranges longer than one day. The CLI splits the range. `--max-days` defaults to 31.
- Fill rate compares booked bay time with published opening hours. It is not official sales history.
- `--estimate-list-price` prices booked slots from the public rate card. A $0 member rate is not membership revenue. Unlisted options are excluded. The result is not a sales record.

## Command Reference

**venue** — Public areas, bays, hours, and guest list prices.

- `your-golf-booking-pp-cli venue --venue forged-golf-club --json`

**occupancy** — Available and booked slots for a date range. Defaults to the venue's local today.

- `your-golf-booking-pp-cli occupancy --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators --from 2026-11-12 --to 2026-11-12 --json`

**summary** — Fill rate by date, weekday, and local hour. Defaults to the seven local days ending today. quiet is under 0.25, moderate is under 0.60, busy is 0.60 or more.

- `your-golf-booking-pp-cli summary --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators --from 2026-11-06 --to 2026-11-12 --bookable simulators --estimate-list-price --json`

**doctor** — Confirm the public page and one local day of bookings respond.

- `your-golf-booking-pp-cli doctor --venue forged-golf-club --json`

## Recipes

Text output lists booked slots. Add `--all-slots` to list available cells too.

```bash
your-golf-booking-pp-cli occupancy --venue forged-golf-club --bookable simulators --from 2026-11-01 --to 2026-11-07 --all-slots
```

Preview the request plan without sending it:

```bash
your-golf-booking-pp-cli summary --venue forged-golf-club --from 2026-11-06 --to 2026-11-12 --dry-run
```

Keep a smaller JSON document:

```bash
your-golf-booking-pp-cli summary --venue forged-golf-club --agent --select totals,summary
```

Raise the date cap only when you mean to:

```bash
your-golf-booking-pp-cli occupancy --venue forged-golf-club --from 2026-10-01 --to 2026-11-12 --max-days 45 --rate-limit 1 --timeout 30s --json
```
