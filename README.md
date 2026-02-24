# AdvancedMD Token Management Service

A high-performance Go microservice that handles AdvancedMD's 2-step authentication flow and caches tokens in Redis. Designed for integration with ElevenLabs conversational agents.

## Features

- **Fast**: Sub-millisecond server processing (~100µs), ~300ms total round-trip
- **Cached**: Tokens stored in Redis with 23-hour TTL
- **Automated**: Background goroutine refreshes tokens every 20 hours
- **Fallback**: On-demand token refresh if cache is empty
- **Reliable**: Graceful shutdown, health checks, request logging
- **Secure**: API key authentication on all endpoints

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Railway                                  │
│  ┌─────────────────┐      ┌─────────────────┐                   │
│  │  Background     │──────│     Redis       │                   │
│  │  Refresh        │      │  (token cache)  │                   │
│  │  (every 20 hrs) │      └─────────────────┘                   │
│  └─────────────────┘              │                             │
│          │                        │                             │
│  ┌───────┴─────────────────────────┴───────┐                    │
│  │              Go Gateway                  │                    │
│  │  • GET  /health              (no auth)  │                    │
│  │  • POST /api/token           (auth req) │                    │
│  │  • POST /api/verify-patient  (auth req) │                    │
│  │  • POST /api/add-patient     (auth req) │                    │
│  │  • POST /api/scheduler/availability     │                    │
│  └─────────────────────────────────────────┘                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                      ┌───────┴───────┐
                      │  ElevenLabs   │
                      │  Agent        │
                      └───────────────┘
```

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
│   │   └── scheduler.go         # Scheduler models + availability
│   ├── auth/
│   │   ├── authenticator.go     # 2-step AdvancedMD authentication
│   │   └── token_manager.go     # Background refresh + caching
│   ├── clients/
│   │   ├── redis.go             # Pooled Redis client
│   │   ├── advancedmd_xmlrpc.go # XMLRPC client (patients, scheduler setup)
│   │   └── advancedmd_rest.go   # REST client (appointments)
│   └── http/
│       ├── router.go            # chi router setup
│       ├── handlers.go          # Request handlers
│       └── middleware.go        # Auth, logging, request ID
│   └── workspace/
│       ├── workspace.go           # go:embed loader for prompt files
│       └── files/                 # Embedded workspace MD files
│           ├── IDENTITY.md        # Agent identity
│           ├── SOUL.md            # Personality + boundaries
│           ├── KNOWLEDGE.md       # Practice info (Abita Eye)
│           ├── TOOLS.md           # API tool instructions
│           ├── USER.md            # Caller context
│           └── VOICE.md           # Speaking style
├── Dockerfile                   # Multi-stage build for Railway
├── go.mod
└── README.md
```

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/Data-Buddies-Solutions/advancedmd-token-management.git
cd advancedmd-token-management
```

### 2. Configure Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `ADVANCEDMD_USERNAME` | Your AdvancedMD API username | `DBSAPI` |
| `ADVANCEDMD_PASSWORD` | Your AdvancedMD API password | `yourpassword` |
| `ADVANCEDMD_OFFICE_KEY` | Your office key | `991NNN` |
| `ADVANCEDMD_APP_NAME` | Your registered app name | `YourAppName` |
| `REDIS_URL` | Redis connection string | `redis://default:pass@host:port` |
| `API_SECRET` | Secret for API authentication | `random-string-456` |
| `PORT` | Server port (optional) | `8080` |

### 3. Run Locally

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

### 4. Deploy to Railway

**Option A: Railway CLI**
```bash
railway login
railway init
railway up
```

**Option B: Railway Dashboard**
1. Go to https://railway.app/new
2. Select "Deploy from GitHub repo"
3. Choose your repository
4. Add environment variables in the Variables tab
5. Railway auto-detects the Dockerfile and deploys

## API Endpoints

### GET /health

Health check endpoint (no authentication required).

```bash
curl https://your-app.railway.app/health
```

**Response:**
```json
{"status":"ok"}
```

### POST /api/token (Precall Webhook)

ElevenLabs precall webhook endpoint. Returns AMD authentication tokens **and** workspace prompt files as dynamic variables in a single call.

**Request:**
```bash
curl -X POST \
     -H "Authorization: Bearer YOUR_API_SECRET" \
     https://your-app.railway.app/api/token
```

**Response:**
```json
{
  "type": "conversation_initiation_client_data",
  "dynamic_variables": {
    "amd_token": "Bearer 991NNN...",
    "amd_rest_api_base": "providerapi.advancedmd.com/api/api-101/YOURAPP",
    "patient_verified": "not_found",
    "patient_id": "1",
    "identity": "[contents of IDENTITY.md]",
    "soul": "[contents of SOUL.md]",
    "user_context": "[contents of USER.md]",
    "tools": "[contents of TOOLS.md]",
    "voice": "[contents of VOICE.md]"
  }
}
```

**Dynamic Variables:**

| Variable | Description |
|----------|-------------|
| `amd_token` | Pre-formatted Bearer token for REST API `Authorization` header |
| `amd_rest_api_base` | REST API base path (use as `https://{amd_rest_api_base}/endpoint`) |
| `patient_verified` | Initial patient state (`not_found`) |
| `patient_id` | Initial patient ID placeholder |
| `identity` | Agent identity prompt (from `IDENTITY.md`) |
| `soul` | Personality and boundaries prompt (from `SOUL.md`) |
| `user_context` | Caller context prompt (from `USER.md`) |
| `tools` | API tool instructions prompt (from `TOOLS.md`) |
| `voice` | Speaking style prompt (from `VOICE.md`) |

The workspace files are embedded into the Go binary at build time using `go:embed`, so no filesystem access is needed at runtime.

> **Note for ElevenLabs:** URLs are returned WITHOUT the `https://` prefix so they can be used as template variables (e.g., `https://{amd_rest_api_base}/scheduler/Columns/openings`).

### POST /api/verify-patient

Looks up a patient by last name and date of birth.

**Request:**
```bash
curl -X POST \
     -H "Authorization: Bearer YOUR_API_SECRET" \
     -H "Content-Type: application/json" \
     -d '{"lastName":"Smith","dob":"01/15/1980"}' \
     https://your-app.railway.app/api/verify-patient
```

**Request Body:**
```json
{
  "lastName": "Smith",
  "dob": "01/15/1980",
  "firstName": "John"  // optional, for disambiguation
}
```

**Response (single match):**
```json
{
  "status": "verified",
  "patientId": "12345",
  "name": "SMITH,JOHN",
  "dob": "01/15/1980",
  "phone": "555-123-4567",
  "insuranceCarrier": "HUMANA MEDICARE",
  "insuranceCarrierId": "car40906",
  "routing": "bach_only",
  "allowedProviders": ["Dr. Bach"],
  "routingAmbiguous": false
}
```

> **Note:** Insurance data is populated by calling AdvancedMD's `getdemographic` API (with `class="demographics"`) after patient verification. The `routing` field determines which providers the patient can see based on their insurance. If `routingAmbiguous` is `true`, the carrier ID is shared across plans and the agent should ask a clarifying question.

**Response (multiple matches):**
```json
{
  "status": "multiple_matches",
  "message": "Found 2 patients with that last name and DOB. Please provide first name.",
  "matches": [
    {"firstName": "JOHN"},
    {"firstName": "JANE"}
  ]
}
```

**Response (not found):**
```json
{
  "status": "not_found",
  "message": "No patient found with that last name and date of birth"
}
```

### POST /api/add-patient

Creates a new patient in AdvancedMD and attaches insurance. Makes two sequential XMLRPC calls: `addpatient` then `addinsurance`.

**Request:**
```bash
curl -X POST \
     -H "Authorization: Bearer YOUR_API_SECRET" \
     -H "Content-Type: application/json" \
     -d '{"firstName":"John","lastName":"Smith","dob":"01/15/1990","email":"john@example.com","phone":"8015551234","street":"123 Main St","city":"Spring Hill","state":"FL","zip":"34609","sex":"male","insurance":"Humana Medicare","subscriberName":"John Smith","subscriberNum":"H12345678"}' \
     https://your-app.railway.app/api/add-patient
```

**Request Body:**
```json
{
  "firstName": "John",
  "lastName": "Smith",
  "dob": "01/15/1990",
  "phone": "8015551234",
  "email": "john@example.com",
  "street": "123 Main St",
  "aptSuite": "",
  "city": "Spring Hill",
  "state": "FL",
  "zip": "34609",
  "sex": "male",
  "insurance": "Humana Medicare",
  "subscriberName": "John Smith",
  "subscriberNum": "H12345678"
}
```

**Supported Insurance Plans:** 44 accepted plans mapped to carrier IDs and routing rules. See `INSURANCE_CROSSWALK.md` for the full list. The `insurance` field accepts the plan name (case-insensitive, e.g., "Humana Medicare", "Florida Blue", "Aetna").

**Response (success):**
```json
{
  "status": "created",
  "patientId": "6034372",
  "name": "SMITH,JOHN",
  "dob": "01/15/1990",
  "routing": "bach_only",
  "allowedProviders": ["Dr. Bach"],
  "message": "Patient created and insurance attached successfully"
}
```

**Response (partial — patient created but insurance failed):**
```json
{
  "status": "partial",
  "patientId": "6034372",
  "name": "SMITH,JOHN",
  "dob": "01/15/1990",
  "message": "Patient created but insurance failed: ..."
}
```

**Response (error):**
```json
{
  "status": "error",
  "message": "Missing required fields: email, phone"
}
```

| Scenario | HTTP | Status |
|----------|------|--------|
| Missing/invalid input | 400 | `error` |
| Unknown insurance name | 400 | `partial` (patient created, insurance not attached) |
| Insurance not accepted at Spring Hill | 400 | `partial` (patient created, routing rejected) |
| Token retrieval fails | 500 | `error` |
| addpatient fails | 500 | `error` |
| addpatient OK, addinsurance fails | 500 | `partial` (includes patientId) |
| Both succeed | 200 | `created` |

### POST /api/scheduler/availability

Returns available appointment slots for providers. Orchestrates multiple AdvancedMD API calls internally and calculates available slots. If the requested date is fully booked, automatically searches forward up to 14 days to find the next available day.

**Request:**
```bash
curl -X POST \
     -H "Authorization: Bearer YOUR_API_SECRET" \
     -H "Content-Type: application/json" \
     -d '{"date":"2026-02-03","provider":"Bach","office":"spring hill"}' \
     https://your-app.railway.app/api/scheduler/availability
```

**Request Body:**
```json
{
  "date": "2026-02-03",
  "provider": "Bach",
  "office": "spring hill",
  "routing": "bach_only"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `date` | Yes | Date in YYYY-MM-DD format |
| `provider` | No | Filter by provider name (partial match, case-insensitive) |
| `office` | No | Filter by office (e.g., "Spring Hill", "Hollywood", "Crystal River") |
| `routing` | No | Insurance routing rule from verify/add-patient (e.g., `bach_only`, `bach_licht`, `all_three`). Filters providers server-side. |

**Response:**
```json
{
  "searchedDate": "2026-02-03",
  "date": "Wednesday, February 4, 2026",
  "location": "ABITA EYE GROUP SPRING HILL",
  "providers": [
    {
      "name": "Dr. Austin Bach",
      "columnId": 1513,
      "profileId": 620,
      "facility": "ABITA EYE GROUP SPRING HILL",
      "slotDuration": 15,
      "totalAvailable": 28,
      "firstAvailable": "8:00 AM",
      "lastAvailable": "4:45 PM",
      "slots": [
        {
          "time": "8:00 AM",
          "datetime": "2026-02-04T08:00"
        },
        {
          "time": "8:15 AM",
          "datetime": "2026-02-04T08:15"
        }
      ]
    }
  ]
}
```

**Response Fields:**

| Field | Description |
|-------|-------------|
| `searchedDate` | The date originally requested (YYYY-MM-DD) |
| `date` | The date with results (may differ from searchedDate if auto-expanded forward) |
| `columnId` | AMD scheduler column ID - **required for booking** |
| `profileId` | AMD provider profile ID - **required for booking** |
| `slotDuration` | Appointment slot length in minutes |
| `totalAvailable` | Total number of available slots for the day |
| `firstAvailable` | Earliest available time (e.g., "8:00 AM") |
| `lastAvailable` | Latest available time (e.g., "4:45 PM") |
| `slots` | First 5 available time slots (use `totalAvailable` for full count) |
| `slots[].datetime` | ISO format for booking API (e.g., `2026-02-03T09:00`) |

#### How Availability Is Calculated

1. **Fetches scheduler setup** from AMD (`getschedulersetup` XMLRPC action)
2. **Fetches existing appointments** per column (`GET /scheduler/appointments` with `forView=day`)
3. **Fetches block holds** per column (`GET /scheduler/blockholds` with `forView=day`)
4. **Generates time slots** based on provider work hours and interval
5. **Filters out:**
   - **Past slots**: If date is today, slots before now + 30 minutes (Eastern time) are excluded
   - **Block holds**: Lunch, meetings, out-of-office, and other blocked periods (uses AMD's `enddatetime` field for accurate multi-day hold coverage)
   - **Existing appointments**: Slots at or above `maxApptsPerSlot` are excluded
   - **Non-work days**: Provider's workweek schedule is respected
6. **Auto-search forward**: If all providers have zero availability, searches the next day (up to 14 days ahead) until openings are found

#### Provider Filtering

Only the following providers at Spring Hill are exposed (live AMD IDs, updated 2026-02-19):

| Column ID | Provider | Profile ID | Schedule | Max/Slot |
|-----------|----------|------------|----------|----------|
| 1513 | Dr. Austin Bach | 620 | Mon-Fri, 8:00 AM - 5:00 PM, 15-min slots | Unlimited |
| 1551 | Dr. J. Licht | 2064 | Wed-Thu, 9:00 AM - 12:30 PM, 15-min slots | 2 |
| 1550 | Dr. D. Noel | 2076 | Mon-Fri, 8:30 AM - 4:30 PM, 30-min slots | 2 |

Spring Hill facility ID: `1568`

To add/remove providers, edit `AllowedColumns` in `internal/domain/scheduler.go`.

#### Insurance-Based Routing

When the `routing` parameter is provided, the availability endpoint filters providers server-side based on the patient's insurance plan:

| Routing Rule | Allowed Providers |
|-------------|-------------------|
| `not_accepted` | None (should not call availability) |
| `bach_only` | Dr. Bach only |
| `bach_licht` | Dr. Bach + Dr. Licht |
| `all_three` | All 3 providers (default) |

The routing value comes from the `verify-patient` or `add-patient` response. See `INSURANCE_CROSSWALK.md` for the complete plan-to-routing mapping.

#### Booking Appointments

The ElevenLabs agent books directly via the AMD REST API using data from this response:

```
POST https://{amd_rest_api_base}/scheduler/Appointments
Authorization: {amd_token}

{
  "patientid": 5984942,
  "columnid": 1513,
  "profileid": 620,
  "startdatetime": "2026-02-03T09:15",
  "clientdatetime": "2026-02-03T09:00",
  "duration": 15,
  "color": "BLUE",
  "type": [{"id": 1006, "name": "NEW ADULT MEDICAL"}]
}
```

| Field | Source |
|-------|--------|
| `columnid` | `columnId` from availability response |
| `profileid` | `profileId` from availability response |
| `startdatetime` | `datetime` from selected slot |
| `duration` | `slotDuration` from provider |
| `type` | Appointment type: 1006=New Adult, 1004=New Pediatric, 1007=Established Follow Up, 1005=Established Pediatric, 1008=Post Op |

## ElevenLabs Integration

### 1. Create Server Tool

In ElevenLabs Agent settings → Add Tool → Webhook:

| Field | Value |
|-------|-------|
| Name | `get_advancedmd_token` |
| Description | Gets a valid authentication token for AdvancedMD API calls. Call this FIRST before any AdvancedMD requests. |
| Method | GET |
| URL | `https://your-app.railway.app/api/token` |

### 2. Configure Authentication

Add header:
- **Name:** `Authorization`
- **Type:** Secret
- **Value:** `Bearer YOUR_API_SECRET`

### 3. Dynamic Variable Assignment

Map response fields for reuse in your ElevenLabs agent:

| Variable | JSON Path | Purpose |
|----------|-----------|---------|
| `amd_token` | `token` | Pre-formatted Bearer token for REST API Authorization header |
| `amd_cookie_token` | `cookieToken` | Pre-formatted Cookie token for XMLRPC Cookie header |
| `amd_xmlrpc_url` | `xmlrpcUrl` | XMLRPC API path (use as `https://{amd_xmlrpc_url}`) |
| `amd_rest_api_base` | `restApiBase` | REST API base (use as `https://{amd_rest_api_base}/endpoint`) |
| `amd_ehr_api_base` | `ehrApiBase` | EHR API base (use as `https://{amd_ehr_api_base}/endpoint`) |

### 4. System Prompt

Add to your agent's system prompt:

```
When the user asks about patient data, appointments, or medical records:

1. FIRST call get_advancedmd_token to get authentication
2. Use the URL variables with https:// prefix:
   - https://{amd_xmlrpc_url} for XMLRPC operations (addpatient, getpatient)
   - https://{amd_rest_api_base}/scheduler/Columns/openings for scheduling
   - https://{amd_ehr_api_base}/files/documents for EHR documents
3. Include Authorization header with {amd_token} (already includes "Bearer " prefix)
4. Handle errors gracefully

The token is cached for ~23 hours - call once per conversation.
```

### 5. Example: REST API Tool (Scheduling)

Create a server tool for getting appointment openings:

| Field | Value |
|-------|-------|
| Name | `get_openings` |
| Description | Gets available appointment openings from AdvancedMD |
| Method | POST |
| URL | `https://{amd_rest_api_base}/scheduler/Columns/openings` |

**Headers:**
- `Authorization`: `{amd_token}` (dynamic variable - already includes "Bearer " prefix)
- `Content-Type`: `application/json`

### 6. Example: Add Patient Tool (via Middleware)

Create a server tool for adding patients with insurance. This calls the middleware which handles both the `addpatient` and `addinsurance` AMD calls.

| Field | Value |
|-------|-------|
| Name | `add_patient` |
| Description | Creates a new patient and attaches their insurance. Collect first name, last name, date of birth, email, phone number, insurance provider, and subscriber number before calling. |
| Method | POST |
| URL | `https://your-app.railway.app/api/add-patient` |

**Headers:**
- `Authorization`: `Bearer YOUR_API_SECRET`
- `Content-Type`: `application/json`

**Body:**
```json
{
  "firstName": "{{first_name}}",
  "lastName": "{{last_name}}",
  "dob": "{{date_of_birth}}",
  "email": "{{email}}",
  "phone": "{{phone}}",
  "insurance": "{{insurance_name}}",
  "subscriberName": "{{subscriber_name}}",
  "subscriberNum": "{{subscriber_number}}"
}
```

## How It Works

### Token Lifecycle

```
Startup: Load from Redis (if exists) → Start background refresh
         ▼
Request: GET /api/token → Return from memory (~100µs server-side)
         ▼
Hour 20: Background refresh → 2-step AMD login → Update Redis + memory
         ▼
Hour 40: Background refresh → 2-step AMD login → Update Redis + memory
...
```

### AdvancedMD 2-Step Authentication

1. **Step 1 - Get Webserver URL**
   - POST to `partnerlogin.advancedmd.com`
   - Returns "error" (success="0") with redirect URL in response

2. **Step 2 - Get Token**
   - POST to webserver URL from Step 1
   - Returns success="1" with session token

### Connection Pooling

Unlike serverless deployments, Railway runs a persistent process:
- **Redis**: Single pooled connection (10 connections, 2 idle)
- **HTTP**: Shared client with keep-alive for AdvancedMD calls
- **Result**: Faster responses, no cold starts

## AdvancedMD API Types

| | XMLRPC API | EHR REST API | PM REST API |
|---|---|---|---|
| **URL** | Single endpoint | Multiple endpoints | Multiple endpoints |
| **Action** | `@action` in body | HTTP method | HTTP method |
| **Format** | `ppmdmsg` wrapper | Standard JSON | Standard JSON |
| **Auth Header** | `Cookie: {amd_cookie_token}` | `Authorization: {amd_token}` | `Authorization: {amd_token}` |
| **Use Cases** | Patients, scheduling | Documents, files | Profiles, master files |

## Performance

### Benchmarks (January 2026)

| Endpoint | Server Processing | Round-Trip (US) |
|----------|-------------------|-----------------|
| `GET /health` | ~20µs | ~280-360ms |
| `GET /api/token` | ~80-110µs | ~280-350ms |
| `POST /api/verify-patient` | ~700ms | ~700-800ms |
| `POST /api/add-patient` | ~700ms | ~700-800ms |

- **Server processing**: Sub-millisecond for cached token retrieval
- **Round-trip**: Includes network latency to Railway's us-east4 region
- **verify-patient**: Includes AdvancedMD XMLRPC lookuppatient + getdemographic calls

## Security

### API Authentication

| Secret | Purpose | Who Uses It |
|--------|---------|-------------|
| `API_SECRET` | Protects all `/api/*` endpoints | ElevenLabs agent |

**How it works:**
1. Client sends `Authorization: Bearer YOUR_API_SECRET`
2. Middleware validates the secret
3. If invalid → 401 Unauthorized

### Security Summary

- All credentials in environment variables
- API endpoints protected by `API_SECRET`
- Redis connection uses TLS (if provider supports it)
- AdvancedMD credentials never exposed to clients
- Request logging with unique request IDs

## Troubleshooting

### Token endpoint returns 401
- Verify `API_SECRET` is set in environment variables
- Check the `Authorization` header format: `Bearer YOUR_SECRET`

### Authentication fails
- Verify AdvancedMD credentials are correct
- Check `ADVANCEDMD_OFFICE_KEY` format
- Ensure `ADVANCEDMD_APP_NAME` is registered with AdvancedMD

### Redis connection fails
- Verify `REDIS_URL` format: `redis://default:password@host:port`
- Check that your Redis instance allows external connections
- Verify the password is correct

### Container won't start
- Check Railway logs for startup errors
- Verify all required environment variables are set
- Ensure `PORT` is set to `8080` (or Railway's assigned port)

## Development

### Running Tests

```bash
go test ./internal/... -v
```

### Building

```bash
go build -o gateway ./cmd/api
```

### Docker Build

```bash
docker build -t advancedmd-gateway .
docker run -p 8080:8080 --env-file .env advancedmd-gateway
```

## License

MIT

## Support

For issues, please open a GitHub issue or contact support.
