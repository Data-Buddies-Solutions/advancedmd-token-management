# Changelog

## [Unreleased] - 2026-03-05

### Availability Slot Calculation — Separate AMD 4101 / 4186 Conflict Checks

Agent tried to book Dr. Licht at 12:15 PM on a day where Bourque was booked at 12:00 with a 30-min duration (covering 12:00–12:30). AMD returned HTTP 409 with error 4101: "Can't add appointment: Overlaps existing appointment." The scheduler had shown 12:15 as available because it lumped all overlapping appointments into a single count and compared against `maxApptsPerSlot=2`.

AMD enforces two independent booking conflict rules:

| Error | Rule | Meaning |
|-------|------|---------|
| **4101** | Duration overlap | Cannot book inside another appointment's `[start, start+duration)` — hard block, `maxApptsPerSlot` irrelevant |
| **4186** | Same-start capacity | Too many appointments starting at the exact same time — controlled by `maxApptsPerSlot` |

#### Changed

- **`internal/http/handlers.go`** — Split single `countOverlappingAppointments()` into two functions:
  - `hasOverlappingAppointment()` — checks if any appointment from a DIFFERENT start time extends into the slot (AMD 4101). Runs unconditionally, including for unlimited columns (Dr. Bach, `maxApptsPerSlot=0`). Returns bool — hard block.
  - `countSameStartAppointments()` — counts appointments starting at the EXACT same time (AMD 4186). Only checked when `maxApptsPerSlot > 0`.
- **`internal/http/handlers.go`** — Updated check order in `calculateAvailableSlots()`:
  1. Past-slot filter
  2. Block hold check
  3. `hasOverlappingAppointment` → hard block (4101)
  4. `countSameStartAppointments` vs `maxApptsPerSlot` → capacity block (4186)
- **`CLAUDE.md`** — Updated "How It Works" step 4 to document both checks; added quirk #10 explaining 4101 vs 4186
- **`README.md`** — Added "Slot Availability Logic" section documenting the two conflict checks

#### Fixed

- **Unlimited columns (Dr. Bach) now enforce overlap blocking** — Previous code wrapped the entire overlap check inside `if maxAppts > 0`, which skipped duration-overlap detection for unlimited columns. A multi-slot appointment on Dr. Bach's column would leave overlapped slots falsely available.

#### Updated Tests

- **`internal/http/handlers_test.go`** — Replaced `TestCountOverlappingAppointments` with `TestHasOverlappingAppointment` (7 cases including the Licht 12:15 scenario) + `TestCountSameStartAppointments` (4 cases). Updated `TestCalculateAvailableSlots_MultiSlotAppointment` comments.

---

## [Previous] - 2026-03-04

### Insurance Network Consolidation + Alias Map + Prompt Update

Consolidated all insurance plans from plan-specific carrier IDs to parent network carrier IDs (71 plans → 22 carrier IDs). Added alias map so `LookupInsurance` catches common shorthand. Updated TOOLS.md to group insurance names by network with agent guidance. Fixed `addinsurance` storing insurance as tertiary instead of primary.

#### Changed

- **`internal/clients/advancedmd_xmlrpc.go`** — Fixed `@coverage` from `"3"` (tertiary) to `"1"` (primary) in `AddInsurance` payload
- **`internal/domain/insurance.go`** — Reorganized `InsuranceNameMap` from routing-tier grouping to carrier-ID grouping (8 major networks + standalone). Consolidated carrier IDs:
  - iCare (car40907): 11 plans — Aetna Better Health, Aetna Better Health of Florida, Aetna Healthy Kids, Aetna Medicare HMO, Community Care Plan, Florida Community Care, Florida Complete Care, Miami Children's Health Plan, Simply Medicaid, Vivida, Doctors Health Medicare
  - United Healthcare (car40923): 11 plans — all UHC variants + UMR + Preferred Care Partners
  - Envolve (car281245): 8 plans — Ambetter variants, Children's Medical Services, Envolve Vision, Staywell Medicare, Sunshine Medicaid, Wellcare
  - Humana Consolidated (car308175): 8 plans — all Humana + Molina Medicare + Cigna Medicare Advantage + Molina Marketplace
  - Florida Blue (car40897): 6 plans
  - Cigna (car301345): 5 plans
  - Aetna (car40887): 4 plans
  - Tricare (car40921): 3 plans
  - 14 standalone carriers (1 plan each)
- **`internal/domain/patient_test.go`** — Updated 3 stale carrier IDs to match consolidated values, added 2 new test cases for alias matching ("Oscar" → Oscar Health, "Humana" → Humana PPO)
- **`INSURANCE_CROSSWALK.md`** — Rewritten to organize by carrier ID groupings instead of routing tiers. insurance.go is now the source of truth.
- **`README.md`** — Updated insurance routing summary (71 plans, 22 carrier IDs, alias map)

#### Added

- **`internal/domain/insurance.go`** — `InsuranceAliases` map (26 entries) + updated `LookupInsurance` to check aliases as fallback. Catches common shorthand:
  - "Oscar" → "Oscar Health"
  - "Humana" → "Humana PPO"
  - "Blue Cross" / "BCBS" → "Florida Blue"
  - "United" / "UHC" → "United Healthcare"
  - "Tricare" → "Tricare Select"
  - "Medicare" → "Florida Medicare"
  - "Cigna" → "Cigna PPO"
  - + 19 more aliases
- **`internal/domain/insurance.go`** — 16 new plan entries (all `RoutingAll`):
  - Aetna Healthy Kids, Aetna QHP Individual Exchange, Ambetter Select, Ambetter Value, Children's Medical Services, Cigna Miami-Dade Public Schools, Cigna Open Access, Florida Blue Medicare PPO, Florida Blue PPO Federal Employee, Florida Blue PPO Out of State, Florida Community Care, Medicaid, Miami Children's Health Plan, Staywell Medicare, Sunshine Medicaid, Vivida
- **`internal/workspace/TOOLS.md`** — Insurance section restructured from flat 54-name list to network-grouped format with 70 names and agent guidance (when to ask follow-ups for Molina, Aetna EPO; shorthand tips for Oscar, Humana, Blue Cross)

#### Note

Molina Medicaid also uses iCare network per the insurance list but retains its own carrier ID (`car40912`).

---

## [Previous] - 2026-03-03

### No-Availability Response Guard

When the 14-day auto-search exhausts without finding any open slots, the availability endpoint previously returned the last searched date with `totalAvailable: 0` and empty `slots`. This allowed the LLM to interpret the response as a valid bookable date. Now returns an explicit no-availability response with empty `date`, empty `providers`, and a `message` field.

#### Changed

- **`internal/domain/scheduler.go`** — Added `Message` field (`omitempty`) to `AvailabilityResponse` struct
- **`internal/http/handlers.go`** — After the 14-day search loop, checks if any provider has availability. If none, returns early with empty `date`, empty `providers`, and `"No availability found within 14 days of requested date"` message

#### Added

- **`internal/http/handlers_test.go`** — 4 new tests:
  - `TestCalculateAvailableSlots_AllBlocked` — full-day block hold → 0 slots
  - `TestCalculateAvailableSlots_AllBookedAtMax` — all slots at max capacity → 0 slots
  - `TestNoAvailabilityResponse_HasMessageAndEmptyProviders` — verifies no-availability JSON structure
  - `TestAvailabilityResponse_OmitsMessageWhenEmpty` — verifies `message` omitted when availability exists

---

### Pediatric Routing — Age-Based Provider Override

Patients under 18 are now automatically routed to Dr. Bach (`bach_only`), the only provider who sees pediatrics. Override is applied server-side after insurance routing, and does not override `not_accepted` insurance.

#### Added

- **`internal/domain/patient.go`** — `IsMinor(dob)` function: parses MM/DD/YYYY DOB, returns true if under 18
- **`internal/domain/patient_test.go`** — `TestIsMinor` with 7 cases (adult, child, exactly 18, turns 18 tomorrow, turned 18 yesterday, invalid, empty)

#### Changed

- **`internal/http/handlers.go`** — Pediatric override in 3 spots:
  - verify-patient (single match): overrides routing to `bach_only` + clears `routingAmbiguous`
  - verify-patient (disambiguation match): same override
  - add-patient (success response): overrides `insEntry.Routing` before building response

---

## [Previous] - 2026-02-24

### Insurance Crosswalk — Server-Side Provider Routing

Replaced the generic 7-carrier map (test-environment IDs) with 44 plan-specific entries using live Spring Hill carrier IDs. Insurance routing is now enforced server-side on the availability endpoint.

#### Added

- **`internal/domain/insurance.go`** — New file with all routing logic:
  - `RoutingRule` type with 4 tiers: `not_accepted`, `bach_only`, `bach_licht`, `all_three`
  - `InsuranceNameMap` — 44 insurance plan names → carrier ID + routing rule
  - `CarrierRoutingMap` — carrier ID → routing rule for existing patients (unambiguous carriers only)
  - `AmbiguousCarriers` — 5 shared carrier IDs that span multiple routing tiers
  - `ColumnsForRouting()` — returns allowed column IDs for a routing rule
  - `ProvidersForRouting()` — returns display names for a routing rule
  - `LookupInsurance()` — normalized name lookup for new patients
  - `RoutingForCarrierID()` — returns routing + ambiguity flag for existing patients
  - `ParseRoutingRule()` — parses routing string from request param

- **`verify-patient` response** — New fields: `insuranceCarrierId`, `routing`, `allowedProviders`, `routingAmbiguous`

- **`add-patient` response** — New fields: `routing`, `allowedProviders`

- **`availability` request** — New `routing` parameter filters columns server-side before AMD API calls

#### Changed

- **`internal/domain/patient.go`** — Removed `CarrierMap`, `LookupCarrierID`, `ValidCarrierNames` (replaced by `insurance.go`)

- **`internal/clients/advancedmd_xmlrpc.go`** — `GetDemographic` now returns `(carrierName, carrierID, error)` instead of `(string, error)`

- **`internal/http/handlers.go`**:
  - verify-patient: Populates routing fields from `RoutingForCarrierID()`
  - add-patient: `carrierId` field → `insurance` field; uses `LookupInsurance()` for carrier ID + routing; rejects `not_accepted` insurance
  - availability: Applies `ColumnsForRouting()` filter before fetching AMD data

- **`internal/workspace/files/TOOLS.md`** — Updated verify_patient (routing fields), add_patient (44-name insurance list, `insurance` field), get_availability (`routing` parameter)

#### Fixed

- **`internal/domain/scheduler_test.go`** — Updated stale Spring Hill facility IDs from test env (`1032`) to live (`1568`)
- **`internal/domain/patient_test.go`** — Replaced `TestLookupCarrierID` with `TestLookupInsurance`

---

## [Previous] - 2026-02-19

### Live AMD Keys for Spring Hill

Updated all hardcoded IDs from the test environment to live AMD system (office 139464).

#### Changed

- **AllowedColumns** (`domain/scheduler.go`) — Replaced test column IDs with live Spring Hill columns:
  - Dr. Bach: `1716` → `1513` (profile `1135` → `620`)
  - Dr. Licht: `1723` → `1551` (profile `1141` → `2064`)
  - Dr. Noel: `1726` → `1550` (profile `1137` → `2076`)
  - Removed all non-Spring Hill columns (Hollywood, Sweetwater, Crystal River)

- **Spring Hill facility ID** (`domain/scheduler.go`) — `1032` → `1568`

- **Provider display names** (`http/handlers.go`) — Updated profile ID keys to match live system

- **Booking payload example** (`README.md`) — Updated `columnid`, `profileid`, and `type` format to match live AMD

#### Added

- **`INSURANCE_MAPPING.md`** — Complete insurance-to-provider routing reference for Spring Hill, derived from the Abita Insurance List PDF (rev 9/4/2025) and validated against live AMD carrier data

#### Discovered

- **`getdemographic`** (class=demographics) returns full patient record including insurance (`insplanlist`) and carrier details (`carrierlist`)
- **`lookupcarrier`** (class=api) returns the practice's carrier master list — searchable by name prefix
- **`getappttypes`** (class=masterfiles) returns appointment types when `appttype` field and `@msgtime` are included
- **Live appointment type IDs**: 1006 (New Adult), 1004 (New Pediatric), 1007 (Established Follow Up), 1005 (Established Pediatric), 1008 (Post Op)
- **MaxApptsPerSlot**: Licht and Noel allow 2 per slot in live (was 0 in test). Bach is 0 (unlimited). Current code treats 0 as 1 — may need revisiting.

---

## [Previous] - 2026-02-16

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

#### Fixed

- **Multi-day block holds now fully block all covered days** — AMD's `duration` field on block holds is unreliable for multi-day holds (e.g., a 4-day "OUT OF OFFICE" hold returns `duration: 510` which only covers 8.5 hours, leaving end-of-day slots falsely available). Now uses AMD's `enddatetime` field instead of computing end from `startdatetime + duration`. Previously, a provider marked out Feb 17-20 would still show 4:30/4:45 PM as available on those days.

#### Discovered

- **AMD requires `columnId`** on `/scheduler/appointments` and `/scheduler/blockholds` — bulk calls without it return HTTP 400. Per-column calls remain necessary.

- **AMD block hold `duration` is unreliable** — For multi-day holds, the `duration` field varies depending on which day you query and doesn't consistently cover the provider's full work hours. The `enddatetime` field is the source of truth.

#### Files Modified

| File | Summary |
|------|---------|
| `internal/domain/scheduler.go` | Updated `AvailableSlot`, `ProviderAvailability`, `AvailabilityResponse` structs; removed `FormatSlotDate`; `BlockHold` now uses `EndDateTime` instead of `Duration`; `IsBlockedByHold` uses `EndDateTime` directly |
| `internal/clients/advancedmd_rest.go` | Changed `forView=week` → `forView=day`; parse `enddatetime` from AMD block hold response |
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
