# AdvancedMD Token Management Service

A high-performance Go microservice deployed on Vercel that handles AdvancedMD's 2-step authentication flow and caches tokens in Redis. Designed for integration with ElevenLabs conversational agents.

## Features

- **Fast**: Go runtime with ~50ms cold starts on Vercel
- **Cached**: Tokens stored in Redis with 23-hour TTL
- **Automated**: Vercel Cron refreshes tokens every 20 hours
- **Fallback**: On-demand token refresh if cache is empty
- **Secure**: API key authentication on all endpoints

## Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│  Vercel Cron    │──────│  /api/cron      │──────│     Redis       │
│  (every 20 hrs) │      │  (Go)           │      │  (token cache)  │
└─────────────────┘      └─────────────────┘      └─────────────────┘
                                                          │
                         ┌─────────────────┐              │
                         │  /api/token     │──────────────┘
                         │  (Go ~50ms)     │
                         └─────────────────┘
                                 │
                         ┌─────────────────┐
                         │  ElevenLabs     │
                         │  Agent          │
                         └─────────────────┘
```

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/Data-Buddies-Solutions/advancedmd-token-management.git
cd advancedmd-token-management
```

### 2. Configure Environment Variables

In Vercel Dashboard → Settings → Environment Variables, add:

| Variable | Description | Example |
|----------|-------------|---------|
| `ADVANCEDMD_USERNAME` | Your AdvancedMD API username | `DBSAPI` |
| `ADVANCEDMD_PASSWORD` | Your AdvancedMD API password | `yourpassword` |
| `ADVANCEDMD_OFFICE_KEY` | Your office key | `991NNN` |
| `ADVANCEDMD_APP_NAME` | Your registered app name | `YourAppName` |
| `REDIS_URL` | Redis connection string | `redis://default:pass@host:port` |
| `CRON_SECRET` | Random secret for cron endpoint | `random-string-123` |
| `API_SECRET` | Random secret for token endpoint | `random-string-456` |

### 3. Deploy to Vercel

```bash
# Install Vercel CLI
npm i -g vercel

# Deploy
vercel --prod
```

Or connect your GitHub repo to Vercel for automatic deployments.

## API Endpoints

### GET /api/token

Returns a valid AdvancedMD session token. Called by ElevenLabs agents.

**Request:**
```bash
curl -H "Authorization: Bearer YOUR_API_SECRET" \
     https://your-app.vercel.app/api/token
```

**Response:**
```json
{
  "token": "Bearer 991NNNzxrAdklblLlx2CAZrB9H1+Grco7wa1Vmxmpo...",
  "cookieToken": "token=991NNNzxrAdklblLlx2CAZrB9H1+Grco7wa1Vmxmpo...",
  "webserverUrl": "providerapi.advancedmd.com/processrequest/api-101/YOURAPP",
  "xmlrpcUrl": "providerapi.advancedmd.com/processrequest/api-101/YOURAPP/xmlrpc/processrequest.aspx",
  "restApiBase": "providerapi.advancedmd.com/api/api-101/YOURAPP",
  "ehrApiBase": "providerapi.advancedmd.com/ehr-api/api-101/YOURAPP",
  "createdAt": "2024-01-09T10:00:00Z"
}
```

**Response Fields:**

| Field | Description | Use Case |
|-------|-------------|----------|
| `token` | Pre-formatted Bearer token | Use directly as `Authorization: {amd_token}` header for REST APIs |
| `cookieToken` | Pre-formatted Cookie token | Use directly as `Cookie: {amd_cookie_token}` header for XMLRPC APIs |
| `webserverUrl` | Base path from login (no https://) | Reference only |
| `xmlrpcUrl` | XMLRPC endpoint path (no https://) | Use as `https://{amd_xmlrpc_url}` |
| `restApiBase` | Practice Manager REST base (no https://) | Use as `https://{amd_rest_api_base}/endpoint` |
| `ehrApiBase` | EHR REST API base (no https://) | Use as `https://{amd_ehr_api_base}/endpoint` |
| `createdAt` | Token generation timestamp | For debugging/monitoring token freshness |

> **Note for ElevenLabs:** URLs are returned WITHOUT the `https://` prefix so they can be used as template variables (e.g., `https://{amd_rest_api_base}/scheduler/Columns/openings`). Two token formats are provided: `token` with "Bearer " prefix for REST API Authorization headers, and `cookieToken` with "token=" prefix for XMLRPC Cookie headers.

### GET /api/cron

Refreshes the token. Triggered automatically by Vercel Cron every 20 hours.

**Request:**
```bash
curl -H "Authorization: Bearer YOUR_CRON_SECRET" \
     https://your-app.vercel.app/api/cron
```

**Response:**
```json
{
  "success": true,
  "message": "Token refreshed successfully",
  "webserverUrl": "https://providerapi.advancedmd.com/processrequest/api-101/YOURAPP"
}
```

## ElevenLabs Integration

### 1. Create Server Tool

In ElevenLabs Agent settings → Add Tool → Webhook:

| Field | Value |
|-------|-------|
| Name | `get_advancedmd_token` |
| Description | Gets a valid authentication token for AdvancedMD API calls. Call this FIRST before any AdvancedMD requests. |
| Method | GET |
| URL | `https://your-app.vercel.app/api/token` |

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
| `amd_webserver` | `webserverUrl` | Base path (reference only, no https://) |
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

### 6. Example: XMLRPC API Tool (Add Patient)

Create a server tool for adding patients:

| Field | Value |
|-------|-------|
| Name | `add_patient` |
| Description | Adds a new patient to AdvancedMD |
| Method | POST |
| URL | `https://{amd_xmlrpc_url}` |

**Headers:**
- `Cookie`: `{amd_cookie_token}` (dynamic variable - pre-formatted with "token=" prefix)
- `Content-Type`: `application/json`

**Body (example):**
```json
{
  "ppmdmsg": {
    "@action": "addpatient",
    "@class": "api",
    "@msgtime": "{{current_datetime}}",
    "@nocookie": "0",
    "patientlist": {
      "patient": {
        "@name": "{{patient_last_name}},{{patient_first_name}}",
        "@sex": "{{patient_sex}}",
        "@dob": "{{patient_dob}}"
      }
    }
  }
}
```

## How It Works

### Token Lifecycle

```
Hour 0:  Cron runs → 2-step AMD login → Token saved (23hr TTL)
         ▼
Hour 1:  ElevenLabs calls /api/token → Redis read (~50ms) ✓
Hour 2:  ElevenLabs calls /api/token → Redis read (~50ms) ✓
...
Hour 19: ElevenLabs calls /api/token → Redis read (~50ms) ✓
         ▼
Hour 20: Cron runs → 2-step AMD login → NEW Token saved
         ▼
Hour 21: ElevenLabs calls /api/token → Redis read (~50ms) ✓
```

### AdvancedMD 2-Step Authentication

1. **Step 1 - Get Webserver URL**
   - POST to `partnerlogin.advancedmd.com`
   - Returns "error" (success="0") with redirect URL in response

2. **Step 2 - Get Token**
   - POST to webserver URL from Step 1
   - Returns success="1" with session token

### Using the Token

The token is pre-formatted with "Bearer " prefix for direct use:
- **Authorization Header:** `Authorization: {amd_token}` (sends `Bearer <token>`)

> **Note:** The token value already includes "Bearer " so don't add it again!

## Project Structure

```
advancedmd-token-management/
├── api/
│   ├── cron.go          # Token refresh endpoint (Vercel Cron)
│   └── token.go         # Token retrieval endpoint (ElevenLabs)
├── pkg/
│   ├── advancedmd/
│   │   └── auth.go      # 2-step authentication logic
│   └── redis/
│       └── redis.go     # Redis client
├── go.mod
├── vercel.json          # Vercel config + cron schedule
└── README.md
```

## Performance

| Metric | Value |
|--------|-------|
| Cold Start | ~50ms |
| Warm Response | ~10-20ms |
| Redis Latency | ~20ms |
| Token TTL | 23 hours |
| Cron Schedule | Every 20 hours |

## Security

### Why API Secrets?

| Secret | Purpose | Who Uses It |
|--------|---------|-------------|
| `API_SECRET` | Protects `/api/token` endpoint from unauthorized access | ElevenLabs agent (you configure this in the tool's Authorization header) |
| `CRON_SECRET` | Protects `/api/cron` endpoint so only Vercel Cron can trigger token refresh | Vercel Cron (automatically sent by Vercel) |

**Without these secrets:**
- Anyone could call your `/api/token` endpoint and get your AdvancedMD credentials
- Anyone could spam your `/api/cron` endpoint, causing unnecessary API calls to AdvancedMD

**How they work:**
1. When ElevenLabs calls `/api/token`, it sends `Authorization: Bearer YOUR_API_SECRET`
2. Your Go function checks if the secret matches before returning the token
3. If it doesn't match → 401 Unauthorized

### Security Summary

- All credentials in Vercel Environment Variables (encrypted at rest)
- Cron endpoint protected by `CRON_SECRET`
- Token endpoint protected by `API_SECRET`
- Redis connection uses TLS (if your provider supports it)
- AdvancedMD credentials never exposed to clients

## AdvancedMD API Types

AdvancedMD has multiple API types. Depending on the operation, you may need to use different APIs:

### XMLRPC API (Legacy/Core Features)

Used for core operations like `addpatient`, `getpatient`, scheduling, etc.

**URL Pattern:** `{webserverUrl}/xmlrpc/processrequest.aspx`

**Request Format:** Uses `ppmdmsg` wrapper with `@action` field
```json
{
  "ppmdmsg": {
    "@action": "addpatient",
    "@class": "api",
    "@msgtime": "4/1/2021 2:16:55 PM",
    "@nocookie": "0",
    "patientlist": {
      "patient": {
        "@name": "Smith,John",
        "@sex": "M",
        "@dob": "01/15/1980"
      }
    }
  }
}
```

**Headers Required:**
- `Cookie: {amd_cookie_token}` (pre-formatted, includes "token=" prefix)
- `Content-Type: application/json`

### EHR REST API (Electronic Health Records)

Used for EHR-specific operations like documents, files, etc.

**URL Pattern:** Replace `processrequest` with `ehr-api` in webserverUrl, then add endpoint path

Example:
- webserverUrl: `https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP`
- EHR Base: `https://providerapi.advancedmd.com/ehr-api/api-801/YOURAPP`
- Full URL: `https://providerapi.advancedmd.com/ehr-api/api-801/YOURAPP/files/documents`

**Request Format:** Standard REST with JSON body (no `ppmdmsg` wrapper)

### Practice Manager REST API

Used for practice management operations like profiles, master files, etc.

**URL Pattern:** Replace `processrequest` with `api` in webserverUrl, then add endpoint path

Example:
- webserverUrl: `https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP`
- REST Base: `https://providerapi.advancedmd.com/api/api-801/YOURAPP`
- Full URL: `https://providerapi.advancedmd.com/api/api-801/YOURAPP/masterfiles/olsprofiles`

### API Comparison

| | XMLRPC API | EHR REST API | PM REST API |
|---|---|---|---|
| **URL** | Single endpoint | Multiple endpoints | Multiple endpoints |
| **Action** | `@action` in body | HTTP method | HTTP method |
| **Format** | `ppmdmsg` wrapper | Standard JSON | Standard JSON |
| **Use Cases** | Patients, scheduling | Documents, files | Profiles, master files |

---

## Recent Updates

### ElevenLabs-Optimized Response Format

The `/api/token` endpoint returns values optimized for ElevenLabs dynamic variables:

```json
{
  "token": "Bearer 991NNN...",
  "cookieToken": "token=991NNN...",
  "webserverUrl": "providerapi.advancedmd.com/processrequest/api-801/YOURAPP",
  "xmlrpcUrl": "providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx",
  "restApiBase": "providerapi.advancedmd.com/api/api-801/YOURAPP",
  "ehrApiBase": "providerapi.advancedmd.com/ehr-api/api-801/YOURAPP",
  "createdAt": "..."
}
```

**Key optimizations for ElevenLabs:**

1. **Token includes "Bearer " prefix** - Use directly as Authorization header value for REST APIs
2. **CookieToken includes "token=" prefix** - Use directly as Cookie header value for XMLRPC APIs
3. **URLs exclude "https://" prefix** - Use in URL templates like `https://{amd_rest_api_base}/endpoint`

**Why this format:** ElevenLabs dynamic variables don't support string concatenation. These formats allow direct use:
- `Authorization: {amd_token}` → sends `Authorization: Bearer <token>` (REST APIs)
- `Cookie: {amd_cookie_token}` → sends `Cookie: token=<token>` (XMLRPC APIs)
- `https://{amd_rest_api_base}/scheduler/Columns/openings` → builds complete URL

### URL Building Logic

The service automatically builds URLs from the webserver URL returned by AdvancedMD:

| URL Type | Transformation | Example Output |
|----------|----------------|----------------|
| `xmlrpcUrl` | Append `/xmlrpc/processrequest.aspx`, strip https:// | `providerapi.advancedmd.com/.../xmlrpc/processrequest.aspx` |
| `restApiBase` | Replace `/processrequest/` with `/api/`, strip https:// | `providerapi.advancedmd.com/api/api-801/APP` |
| `ehrApiBase` | Replace `/processrequest/` with `/ehr-api/`, strip https:// | `providerapi.advancedmd.com/ehr-api/api-801/APP` |

---

## Troubleshooting

### Token endpoint returns 401
- Verify `API_SECRET` is set in Vercel environment variables
- Check the `Authorization` header format: `Bearer YOUR_SECRET`

### Authentication fails
- Verify AdvancedMD credentials are correct
- Check `ADVANCEDMD_OFFICE_KEY` format
- Ensure `ADVANCEDMD_APP_NAME` is registered with AdvancedMD

### Redis connection fails
- Verify `REDIS_URL` format: `redis://default:password@host:port`
- Check that your Redis instance allows external connections
- Verify the password is correct

## License

MIT

## Support

For issues, please open a GitHub issue or contact support.
