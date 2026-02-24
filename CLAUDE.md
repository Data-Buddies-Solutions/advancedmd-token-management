# CLAUDE.md

## Project Purpose

This project exists to **understand and document AdvancedMD APIs** so that **ElevenLabs conversational agents can make tool calls** to interface with AdvancedMD's healthcare practice management system.

The codebase serves two purposes:
1. **Token Management Service**: A Go microservice that handles AdvancedMD's complex 2-step authentication and caches tokens for ElevenLabs agents
2. **API Documentation Reference**: Living documentation of how AdvancedMD APIs work, their quirks, and how to integrate them

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
| **REST (Practice Manager)** | Replace `/processrequest/` with `/api/` | Standard JSON | profiles, master files |
| **EHR REST** | Replace `/processrequest/` with `/ehr-api/` | Standard JSON | documents, files |

### Token Format for ElevenLabs

The `/api/token` endpoint serves as the **precall webhook** for ElevenLabs. It returns both AMD tokens and workspace prompt files as dynamic variables:

- `amd_token`: Includes "Bearer " prefix → Use directly as `Authorization: {amd_token}`
- `amd_rest_api_base`: Excludes "https://" prefix → Use as `https://{amd_rest_api_base}/endpoint`
- `identity`, `soul`, `user_context`, `tools`, `voice`: Workspace prompt files loaded via `go:embed`

This is because ElevenLabs doesn't support string concatenation in dynamic variables.

### Workspace Files

Prompt files live in `internal/workspace/files/` and are embedded into the binary at build time. The `workspace.Variables()` function returns them as a `map[string]string` keyed by ElevenLabs variable name. To update prompts, edit the MD files and redeploy.

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
│   │   └── advancedmd_rest.go   # REST client (appointments)
│   ├── http/
│   │   ├── router.go            # chi router setup
│   │   ├── handlers.go          # Request handlers
│   │   └── middleware.go        # Auth, logging, request ID
│   └── workspace/
│       ├── workspace.go         # go:embed loader for prompt files
│       └── files/               # Embedded MD prompt files
│           ├── IDENTITY.md      # Agent identity
│           ├── SOUL.md          # Personality + boundaries
│           ├── KNOWLEDGE.md     # Practice info (Abita Eye)
│           ├── TOOLS.md         # API tool instructions
│           ├── USER.md          # Caller context
│           └── VOICE.md         # Speaking style
├── Dockerfile                   # Multi-stage build for Railway
└── README.md                    # User-facing documentation
```

## Common Tasks

### Running Locally

```bash
# Set environment variables
export ADVANCEDMD_USERNAME=...
export ADVANCEDMD_PASSWORD=...
export ADVANCEDMD_OFFICE_KEY=...
export ADVANCEDMD_APP_NAME=...
export REDIS_URL=...
export API_SECRET=...

# Build and run
go build -o gateway ./cmd/api && ./gateway
```

### Testing the Token Endpoint

```bash
curl -H "Authorization: Bearer YOUR_API_SECRET" http://localhost:8080/api/token
```

### Deploying to Railway

```bash
railway login
railway link
railway up
```

## Scheduler Availability Endpoint

The `/api/scheduler/availability` endpoint orchestrates multiple AMD API calls to return available appointment slots.

### How It Works

1. Calls `getschedulersetup` (XMLRPC) → Gets provider columns, profiles, facilities
2. Calls `GET /scheduler/appointments` per column (REST, `forView=day`) → Gets existing booked appointments
3. Calls `GET /scheduler/blockholds` per column (REST, `forView=day`) → Gets blocked time periods
4. Calculates available slots based on:
   - Provider work hours (from `columnsetting`)
   - Slot interval (15 or 30 min depending on provider)
   - Existing appointments (respects `maxApptsPerSlot`)
   - **Block holds** from AMD (lunch, meetings, out of office, etc.)
   - Provider workweek (e.g., Dr. Licht only works Wed-Thu)
   - **Past-slot filter**: If date is today, slots before `now + 30 min` Eastern are excluded
5. If ALL providers have zero availability, **auto-searches forward** day-by-day (up to 14 days) until openings are found

### Response Format

The response is optimized for ElevenLabs LLM token efficiency:
- Max **5 slots** returned per provider (with `totalAvailable` count for the full day)
- `firstAvailable` / `lastAvailable` summary fields
- `searchedDate` (original request) vs `date` (actual result — may differ if auto-expanded)
- No redundant `date` field on individual slots (single-day search)
- No `schedule` field (was verbose, not useful for the LLM)

### AMD API Constraint: columnId Required

AMD's `/scheduler/appointments` and `/scheduler/blockholds` endpoints **require `columnId`** — bulk calls without it return HTTP 400. So we make per-column calls (N appointments + N block holds per day searched).

### AMD Response Structure Quirks

The `getschedulersetup` response has prefixed IDs that must be stripped:
- Column IDs: `col1513` → `1513`
- Profile IDs: `prof620` → `620`
- Facility IDs: `fac1568` → `1568`

Times are nested inside `columnsetting`:
```json
{
  "@id": "col1513",
  "@name": "DR. BACH - BP",
  "@profile": "prof620",
  "@facility": "fac1568",
  "columnsetting": {
    "@start": "08:00",
    "@end": "17:00",
    "@interval": "15",
    "@maxapptsperslot": "0",
    "@workweek": "1111100"
  }
}
```

Workweek format: 7 chars for Mon-Sun where `1` = works, `0` = off.

### Allowed Providers (Spring Hill) — LIVE IDs

Updated 2026-02-19 from live AMD system (office 139464).

Only these columns are exposed (edit `AllowedColumns` in `domain/scheduler.go` to change):

| Column ID | Name | Profile ID | Facility ID | Hours | Interval | Max/Slot | Workweek |
|-----------|------|------------|-------------|-------|----------|----------|----------|
| 1513 | DR. BACH - BP | 620 | 1568 | 8:00-17:00 | 15 min | 0 (unlimited) | Mon-Fri |
| 1551 | DR. LICHT | 2064 | 1568 | 9:00-12:30 | 15 min | 2 | Wed-Thu |
| 1550 | DR. NOEL | 2076 | 1568 | 8:30-16:30 | 30 min | 2 | Mon-Fri |

Spring Hill facility ID: **1568** (was 1032 in test env)

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

**Key files:**
- `internal/domain/insurance.go` — `InsuranceNameMap`, `CarrierRoutingMap`, `AmbiguousCarriers`, routing functions
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

## ElevenLabs Integration Notes

When creating ElevenLabs tools that call AdvancedMD:

1. **Always call `get_advancedmd_token` first** to get authentication
2. **Map response fields to dynamic variables**:
   - `amd_token` → `token`
   - `amd_cookie_token` → `cookieToken`
   - `amd_rest_api_base` → `restApiBase`
   - `amd_ehr_api_base` → `ehrApiBase`
   - `amd_xmlrpc_url` → `xmlrpcUrl`
3. **Use correct auth header for API type**:
   - REST APIs: `Authorization: {amd_token}`
   - XMLRPC APIs: `Cookie: {amd_cookie_token}`

## XMLRPC Actions Reference

| Action | Class | Description |
|--------|-------|-------------|
| `lookuppatient` | `api` | Search patients by last name |
| `addpatient` | `api` | Create a new patient |
| `addinsurance` | `api` | Attach insurance to a patient |
| `getdemographic` | `demographics` | Get patient demographics + insurance (must use `demographics` class, not `api`) |
| `lookupcarrier` | `api` | Search insurance carriers (paginated, 50 per page) |

## Future Documentation Goals

- Document each AdvancedMD API endpoint as we use them
- Create example payloads for common operations (scheduling)
- Document error codes and their meanings
- Build out ElevenLabs tool configurations for specific use cases
