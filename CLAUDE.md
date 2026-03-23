# CLAUDE.md

## Project Purpose

This project exists to **understand and document AdvancedMD APIs** so that **ElevenLabs conversational agents can make tool calls** to interface with AdvancedMD's healthcare practice management system.

The codebase is a **Go microservice** (middleware) that handles AdvancedMD's complex 2-step authentication, caches tokens, and provides server-side endpoints for patient verification, registration, insurance routing, and appointment availability.

## Key Concepts

### AdvancedMD Authentication Flow

AdvancedMD uses a non-standard 2-step authentication:

1. **Step 1**: POST to `partnerlogin.advancedmd.com` → Returns a webserver URL (confusingly returns `success="0"` but includes the URL)
2. **Step 2**: POST to the webserver URL → Returns the actual session token

See `internal/auth/authenticator.go` for implementation details.

### AdvancedMD API Types

AdvancedMD has **three different API types**, each with different URL patterns and request formats:

| API Type | URL Pattern | Request Format | Use Cases |
|----------|-------------|----------------|-----------|
| **XMLRPC** | `{webserver}/xmlrpc/processrequest.aspx` | `ppmdmsg` wrapper with `@action` | addpatient, getpatient, getdemographic, scheduling |
| **REST (Practice Manager)** | Replace `/processrequest/` with `/api/` | Standard JSON | appointments, block holds |
| **EHR REST** | Replace `/processrequest/` with `/ehr-api/` | Standard JSON | documents, files |

### Token Format for ElevenLabs

The `/api/token` endpoint serves as the **precall webhook** for ElevenLabs. It returns AMD tokens as dynamic variables:

- `amd_token`: Includes "Bearer " prefix → Use directly as `Authorization: {amd_token}`
- `amd_rest_api_base`: Excludes "https://" prefix → Use as `https://{amd_rest_api_base}/endpoint`
- `patient_id`: Placeholder initial value (`"1"`) — overwritten after verify/add-patient

ElevenLabs doesn't support string concatenation in dynamic variables, which is why URLs exclude the protocol prefix.

### Workspace Files

Prompt files live in `internal/workspace/` and are tracked in git for diff history. They are **not loaded at runtime** — prompts are managed directly in ElevenLabs. The files here are the source of truth for prompt content and change tracking.

- `SOUL.md` — Personality + boundaries
- `TOOLS.md` — API tool instructions for the agent
- `VOICE.md` — Speaking style
- `KNOWLEDGE.md` — Practice info (synced to ElevenLabs knowledge base)
- `CHANGELOG.md` — History of all prompt changes with rationale

## Project Structure

```
advancedmd-token-management/
├── cmd/
│   └── api/
│       └── main.go              # Server entrypoint, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go            # Environment variable loading
│   ├── domain/
│   │   ├── token.go             # Token model + URL transforms
│   │   ├── patient.go           # Patient model + DOB normalization
│   │   ├── insurance.go         # Insurance routing rules + carrier maps
│   │   └── scheduler.go         # Scheduler models + availability logic
│   ├── auth/
│   │   ├── authenticator.go     # 2-step AdvancedMD authentication
│   │   └── token_manager.go     # Background refresh + caching
│   ├── clients/
│   │   ├── redis.go             # Pooled Redis client
│   │   ├── advancedmd_xmlrpc.go # XMLRPC client (patients, scheduler setup)
│   │   └── advancedmd_rest.go   # REST client (appointments, booking, block holds)
│   ├── http/
│   │   ├── router.go            # chi router setup
│   │   ├── handlers.go          # Request handlers
│   │   └── middleware.go        # Auth, logging, request ID
│   └── workspace/               # Agent prompt files (git-tracked only)
│       ├── SOUL.md
│       ├── TOOLS.md
│       ├── VOICE.md
│       ├── KNOWLEDGE.md
│       └── CHANGELOG.md
├── Dockerfile                   # Multi-stage build (Go 1.25, Alpine 3.23)
└── README.md
```

## Common Tasks

### Running Locally

```bash
export ADVANCEDMD_USERNAME=...
export ADVANCEDMD_PASSWORD=...
export ADVANCEDMD_OFFICE_KEY=...
export ADVANCEDMD_APP_NAME=...
export REDIS_URL=...
export API_SECRET=...

go build -o gateway ./cmd/api && ./gateway
```

### Running Tests

```bash
go test ./...
```

### Deploying to Railway

```bash
railway login
railway up
```

## API Endpoints

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /health` | No | Health check |
| `POST /api/token` | Yes | Precall webhook — returns AMD tokens as dynamic variables |
| `POST /api/verify-patient` | Yes | Patient lookup by name + DOB, returns insurance routing |
| `POST /api/add-patient` | Yes | Patient creation + insurance attachment |
| `POST /api/scheduler/availability` | Yes | Available appointment slots (concurrent per-column fetching) |
| `POST /api/patient/appointments` | Yes | Upcoming appointments for a verified patient |
| `POST /api/appointment/book` | Yes | Book appointment (type→color mapping, constants handled server-side) |
| `POST /api/appointment/cancel` | Yes | Cancel an appointment |

## Scheduler Availability Endpoint

The `/api/scheduler/availability` endpoint orchestrates multiple AMD API calls to return available appointment slots.

### How It Works

1. Calls `getschedulersetup` (XMLRPC) → Gets provider columns, profiles, facilities
2. Calls `GET /scheduler/appointments` per column **concurrently** (REST, `forView=day`)
3. Calls `GET /scheduler/blockholds` per column **concurrently** (REST, `forView=day`)
4. Calculates available slots based on:
   - Provider work hours (from `columnsetting`)
   - Slot interval (15 or 30 min depending on provider)
   - **Appointment duration overlap** (AMD 4101): If any appointment's `[start, start+duration)` covers a slot, that slot is hard-blocked — you cannot book inside another appointment's time range
   - **Same-start capacity** (AMD 4186): Multiple appointments can start at the exact same time, up to `maxApptsPerSlot` (0 = unlimited)
   - **Block holds** from AMD (lunch, meetings, out of office, etc.)
   - Provider workweek (e.g., Dr. Licht only works Wed-Thu)
   - **Same-day block**: Requests for today's date (Eastern time) are rejected — same-day appointments are not available
5. If ALL providers have zero availability, **auto-searches forward** day-by-day (up to 14 days) until openings are found

### Response Format

Optimized for ElevenLabs LLM token efficiency:
- Max **5 slots** returned per provider (with `totalAvailable` count for the full day)
- `firstAvailable` / `lastAvailable` summary fields
- `searchedDate` (original request) vs `date` (actual result — may differ if auto-expanded)

### AMD API Constraint: columnId Required

AMD's `/scheduler/appointments` and `/scheduler/blockholds` endpoints **require `columnId`** — bulk calls without it return HTTP 400. Per-column calls are made concurrently (N appointments + N block holds in parallel per day searched).

### AMD Response Structure Quirks

The `getschedulersetup` response has prefixed IDs that must be stripped:
- Column IDs: `col1513` → `1513`
- Profile IDs: `prof620` → `620`
- Facility IDs: `fac1568` → `1568`

Workweek format: 7 chars for Mon-Sun where `1` = works, `0` = off.

### Allowed Providers (Spring Hill) — LIVE IDs

Updated 2026-02-19 from live AMD system (office 139464).

Only these columns are exposed (edit `OfficeRegistry` in `domain/office.go` to change):

| Column ID | Name | Profile ID | Facility ID | Hours | Interval | Max/Slot | Workweek |
|-----------|------|------------|-------------|-------|----------|----------|----------|
| 1513 | DR. BACH - BP | 620 | 1568 | 8:00-17:00 | 15 min | 1 | Mon-Fri |
| 1551 | DR. LICHT | 2064 | 1568 | 9:00-12:30 | 15 min | 2 | Wed-Thu |
| 1550 | DR. NOEL | 2076 | 1568 | 8:30-16:30 | 30 min | 2 | Mon-Fri |

Spring Hill facility ID: **1568**

### Appointment Type IDs (LIVE)

| Type | AMD ID | AMD Name |
|------|--------|----------|
| New Adult Medical | 1006 | NEW ADULT MEDICAL |
| New Pediatric Medical | 1004 | NEW PEDIATRIC MEDICAL |
| Established Adult Medical (Follow Up) | 1007 | ESTABLISH ADULT MEDICAL |
| Established Pediatric Medical (Follow Up) | 1005 | ESTABLISH PEDIATRIC MED |
| Post Op | 1008 | POST OP |

### Insurance Routing

Insurance-based provider routing is enforced server-side. See `INSURANCE_CROSSWALK.md` for the complete reference and `internal/domain/insurance.go` for the implementation.

**How it works:**
- 44 insurance plans mapped to carrier IDs + routing rules in `InsuranceNameMap`
- 4 routing tiers: `not_accepted`, `bach_only`, `bach_licht`, `all_three`
- **Existing patients**: `verify-patient` calls `GetDemographic` → gets carrier ID → `RoutingForCarrierID()` returns routing + ambiguity flag
- **New patients**: `add-patient` receives insurance name from LLM → `LookupInsurance()` returns carrier ID + routing
- **Scheduling**: `get_availability` accepts optional `routing` param → `ColumnsForRouting()` filters columns before any AMD API calls
- 5 ambiguous carrier IDs (Aetna, FL Blue, Molina, UHC, Cigna HMO) default to `all_three` with `routingAmbiguous: true` flag so the agent can ask a clarifying question
- **Pediatric override**: Patients under 18 (via `IsMinor()` in `patient.go`) are automatically routed to `bach_only` regardless of insurance routing. Applied server-side in both `verify-patient` and `add-patient` handlers after insurance routing is determined. Does not override `not_accepted` insurance.

**Key files:**
- `internal/domain/insurance.go` — `InsuranceNameMap`, `CarrierRoutingMap`, `AmbiguousCarriers`, routing functions
- `internal/domain/patient.go` — `IsMinor()` for age-based pediatric routing override
- `INSURANCE_CROSSWALK.md` — Source reference with all 44 plans, routing rules, and shared carrier ID documentation

## AdvancedMD API Quirks to Know

1. **Step 1 returns "error"**: The first login step returns `success="0"` with an error code, but this is expected - the webserver URL is still in the response

2. **XML charset issues**: AdvancedMD may return ISO-8859-1 encoded XML, requiring charset handling (see `parseXMLResponse` in auth.go)

3. **Token in Cookie vs Authorization**:
   - XMLRPC APIs use `Cookie: token={token}`
   - REST APIs use `Authorization: Bearer {token}`

4. **URL transformations**: Different API types require transforming the webserver URL by replacing path segments

5. **getdemographic class matters**: Using `class="api"` omits insurance data entirely. Use `class="demographics"` to get `insplanlist` and `carrierlist` in the response

6. **Carrier IDs**: Insurance name → carrier ID mapping lives in `internal/domain/insurance.go` `InsuranceNameMap` (44 plans). Use `lookupcarrier` XMLRPC action to find new carrier IDs (180 carriers across 4 pages)

7. **Scheduler setup prefixes**: Column, profile, and facility IDs have prefixes (`col`, `prof`, `fac`) that must be stripped

8. **Block hold `duration` is unreliable for multi-day holds**: For multi-day block holds (e.g., "OUT OF THE OFFICE" spanning Feb 17-20), AMD returns a `duration` that doesn't always cover the full day. Use the `enddatetime` field instead of computing end from `startdatetime + duration`. See `IsBlockedByHold` in `domain/scheduler.go`.

9. **AMD single-vs-array responses**: AMD returns a single JSON object when there's one result, but an array when there are multiple. All parsing code must handle both formats (see `AMDLookupResponse` vs `AMDLookupResponseSingle` pattern in `advancedmd_xmlrpc.go`).

10. **AMD appointment conflict errors (409) are two separate checks**:
   - **4101 — Overlaps existing appointment**: Fires when the new appointment's start time falls within another appointment's `[start, start+duration)`. This is a **hard block** — `maxApptsPerSlot` does NOT override it. You cannot book inside another appointment's time range.
   - **4186 — Max appointments per slot exceeded**: Fires when too many appointments share the exact same start time. Controlled by `maxApptsPerSlot` (0 = unlimited).
   - These are independent checks. `maxApptsPerSlot=2` means two appointments can start at 9:00 simultaneously, but you still can't book at 9:15 if a 9:00 appointment has a 30-min duration covering that slot.
   - See `hasOverlappingAppointment()` (4101) and `countSameStartAppointments()` (4186) in `handlers.go`.

## XMLRPC Actions Reference

| Action | Class | Description |
|--------|-------|-------------|
| `lookuppatient` | `api` | Search patients by last name |
| `addpatient` | `api` | Create a new patient |
| `addinsurance` | `api` | Attach insurance to a patient |
| `getdemographic` | `demographics` | Get patient demographics + insurance (must use `demographics` class, not `api`) |
| `getschedulersetup` | `masterfiles` | Get scheduler columns, profiles, facilities |
| `lookupcarrier` | `api` | Search insurance carriers (paginated, 50 per page) |
