# Changelog

## [Unreleased] - 2026-02-16

### Availability Endpoint Improvements

Refactored `/api/scheduler/availability` to produce a cleaner, more token-efficient response for ElevenLabs LLM consumption, filter stale slots, and automatically find the next available day when booked.

#### Changed

- **Cleaner response format** — Response optimized for LLM token efficiency:
  - Slots capped at **5 per provider** (with `totalAvailable` for full count)
  - Added `firstAvailable` / `lastAvailable` summary fields
  - Added `searchedDate` (original request) vs `date` (actual result, may differ if auto-expanded)
  - Removed `date` field from individual slots (redundant for single-day search)
  - Removed `schedule` field from providers (verbose, not useful for the LLM)
  - Renamed `availableSlots` → `slots`

- **Past-slot filtering** — If the requested date is today, slots before `now + 30 minutes` Eastern time are excluded. No more offering 8:00 AM when it's already 2:00 PM.

- **Auto-search forward** — When ALL providers have zero availability on the requested date, the endpoint automatically searches day-by-day up to 14 days ahead and returns the first day with any openings. `searchedDate` shows what was requested; `date` shows what was found.

- **`forView=day` instead of `forView=week`** — REST calls to AMD now use `forView=day` since we search one day at a time, reducing response payload size.

- **Removed `days` request parameter** — The endpoint now always searches a single day (with auto-forward on fully booked days), replacing the old multi-day range approach.

#### Discovered

- **AMD requires `columnId`** on `/scheduler/appointments` and `/scheduler/blockholds` — bulk calls without it return HTTP 400. Per-column calls remain necessary.

#### Files Modified

| File | Summary |
|------|---------|
| `internal/domain/scheduler.go` | Updated `AvailableSlot`, `ProviderAvailability`, `AvailabilityResponse` structs; removed `FormatSlotDate` |
| `internal/clients/advancedmd_rest.go` | Changed `forView=week` → `forView=day` on appointments and block holds calls |
| `internal/http/handlers.go` | Added past-slot filter, auto-search loop, slot cap; removed `buildScheduleDescription`, `formatTimeForDisplay`, `days` parameter |

#### Response Before vs After

**Before** (up to 66 slot objects, verbose):
```json
{
  "date": "Tuesday, February 17, 2026",
  "providers": [{
    "schedule": "Monday-Friday, 8:00 AM - 5:00 PM",
    "availableSlots": [
      {"date": "Tuesday, February 17", "time": "8:00 AM", "datetime": "..."},
      {"date": "Tuesday, February 17", "time": "8:15 AM", "datetime": "..."},
      ...60+ more slots...
    ]
  }]
}
```

**After** (max 5 slots, summary fields):
```json
{
  "searchedDate": "2026-02-17",
  "date": "Tuesday, February 17, 2026",
  "providers": [{
    "totalAvailable": 28,
    "firstAvailable": "8:00 AM",
    "lastAvailable": "4:45 PM",
    "slots": [
      {"time": "8:00 AM", "datetime": "2026-02-17T08:00"},
      {"time": "8:15 AM", "datetime": "2026-02-17T08:15"},
      {"time": "8:30 AM", "datetime": "2026-02-17T08:30"},
      {"time": "8:45 AM", "datetime": "2026-02-17T08:45"},
      {"time": "9:00 AM", "datetime": "2026-02-17T09:00"}
    ]
  }]
}
```
