# Validation — 2026-11-12

Binary built from this tree: `go build -o /tmp/your-golf-booking-pp-cli ./cmd/your-golf-booking-pp-cli`

## Catalog checks

| Check | Result | Command |
|---|---|---|
| unit tests | pass | `go test ./...` |
| vet | pass | `go vet ./...` |
| build | pass | `go build -o /tmp/your-golf-booking-pp-cli ./cmd/your-golf-booking-pp-cli` |
| help | pass | `/tmp/your-golf-booking-pp-cli --help` |
| version | pass | `/tmp/your-golf-booking-pp-cli --version` → `your-golf-booking-pp-cli 0.1.0` |
| skill verifier | pass | `python3 .github/scripts/verify-skill/verify_skill.py --dir library/other/your-golf-booking` |

## Live Forged reads

User-Agent: `your-golf-booking-pp-cli/0.1.0 (read-only public occupancy; +https://github.com/mvanhorn/printing-press-library)`

Doctor, pass:

```bash
your-golf-booking-pp-cli doctor \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators \
  --json
```

Occupancy for the local day 2026-11-12, pass. Simulators only (the URL names that area). Four bays, 30-minute slots, public rate `$30.00` per slot (`$60.00/Hr` on the card). Totals: 192 open slots, 47 booked, 145 available, fill `0.2448` (quiet).

```bash
your-golf-booking-pp-cli occupancy \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators \
  --from 2026-11-12 --to 2026-11-12 --json
```

Summary 2026-11-06 through 2026-11-12, pass. Strongest date Friday 2026-11-06, fill `0.3802` (moderate). Quietest date Sunday 2026-11-08, fill `0.1354` (quiet). Busiest hour 20:00 local, fill `0.7321` (busy). Quietest hour 00:00, fill `0`. List-price estimate `$90.00` across `172.00` booked hours, of which most minutes are a `$0` published bay price or an option that is not on the public rate card. That estimate is not a sales total.

```bash
your-golf-booking-pp-cli summary \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators \
  --from 2026-11-06 --to 2026-11-12 --estimate-list-price --json
```

Past date 2026-08-01, pass: 39 booked slots, fill `0.2031`. The feed returned history rather than an error.

```bash
your-golf-booking-pp-cli summary \
  --venue https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators \
  --from 2026-08-01 --to 2026-08-01 --json
```

## Redacted occupancy sample

Slots below are the public calendar only. Booking ids and player counts are omitted.

```json
{
  "venue": {
    "name": "Forged Golf Club",
    "slug": "forged-golf-club",
    "timezone": "America/Chicago",
    "currency": "usd"
  },
  "bookables": ["simulators"],
  "public_price": {
    "name": "Public Rate",
    "display_price": "$60.00/Hr",
    "per_slot": "$30.00 / 30 minutes"
  },
  "totals": {
    "open_slots": 192,
    "booked_slots": 47,
    "available_slots": 145,
    "fill_rate": 0.2448,
    "band": "quiet"
  },
  "slots": [
    {
      "date": "2026-11-12",
      "start": "2026-11-12T00:00:00-06:00",
      "end": "2026-11-12T00:30:00-06:00",
      "bay_label": "1",
      "state": "available",
      "guest_price_name": "Public Rate",
      "guest_price_display": "$30.00"
    },
    {
      "date": "2026-11-12",
      "start": "2026-11-12T11:30:00-06:00",
      "end": "2026-11-12T12:00:00-06:00",
      "bay_label": "1",
      "state": "booked",
      "status": "confirmed",
      "option_name": "League Member",
      "guest_price_display": "$30.00"
    }
  ]
}
```
