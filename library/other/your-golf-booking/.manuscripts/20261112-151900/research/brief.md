# Your Golf Booking public occupancy — research brief

Checked 2026-11-12 against the live Forged Golf Club booking page and `api.yourgolfbooking.com`. Signed out. No login, checkout, cart, or booking request.

The Printing Press generator is not in this workspace, so this CLI was added on the catalog's manual path and shaped like the published cobra CLIs (`rootFlags`, `--json`, `doctor`, `--version`).

## What the public pages are

TrackMan booking pages on `booking.trackmangolf.com` are Your Golf Booking. The Forged simulator page is:

`https://booking.trackmangolf.com/venues/forged-golf-club/booking/simulators`

`/booking/bays` and `/booking` are the same host. The HTML `__NEXT_DATA__` payload includes `initialReduxState` with the venue, bookable areas, bays, opening hours, and rate card. Forged's simulator area slug is `simulators` (Trackman Bays, 30-minute slots, four bookable bays). PuttView is the `bays` area.

Guest list price on that page: Public Rate, display `$60.00/Hr`, integer price `3000` minor units per 30-minute slot (`$30.00`). Member and perk rates on the same card publish a `$0` bay price. The CLI does not read payment configuration.

The structured address on the page is street `650 E Kensington Rd`, town field `Illinois`, postal code `60004`. The CLI copies those fields as published.

## Bookings feed

`GET https://api.yourgolfbooking.com/venue/{slug}/bookings/public?start_gte=...&start_lte=...`

- HTTP 200 and a JSON array of `{start, end, status, bayRef, bayOptionId, playerOptions[{quantity}]}`.
- No customer name, email, phone, or payment fields in the objects observed.
- A window longer than one day returns HTTP 400: `Cannot pass range greater than a day.` A 24-hour window is accepted.
- Past dates are accepted. `2026-08-01` returned attended bookings. `2026-01-10` returned an empty array and HTTP 200, which means no public bookings in that window, not a refusal. No official retention cutoff was published.
- Statuses observed include `confirmed` and `attended`.

## No venue index

Checked, without crawling the host:

- `GET https://api.yourgolfbooking.com/venues` → 403 Forbidden
- `GET https://api.yourgolfbooking.com/venue` → 404
- `https://booking.trackmangolf.com/sitemap.xml` → 404
- `https://booking.trackmangolf.com/venues` → 404
- `robots.txt` allows all user agents and lists no sitemap

The caller supplies a slug or a public URL.
