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
  "token": "991NNNzxrAdklblLlx2CAZrB9H1+Grco7wa1Vmxmpo...",
  "webserverUrl": "https://providerapi.advancedmd.com/processrequest/api-101/YOURAPP",
  "createdAt": "2024-01-09T10:00:00Z"
}
```

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

Map response fields for reuse:

| Variable | JSON Path |
|----------|-----------|
| `amd_token` | `token` |
| `amd_webserver` | `webserverUrl` |

### 4. System Prompt

Add to your agent's system prompt:

```
When the user asks about patient data, appointments, or medical records:

1. FIRST call get_advancedmd_token to get authentication
2. The token is stored in {amd_token}, server URL in {amd_webserver}
3. Use these in subsequent AdvancedMD API calls
4. Handle errors gracefully

The token is cached for ~23 hours - call once per conversation.
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

In AdvancedMD API calls:
- **Cookie:** `Cookie: token={token}`
- **Or Bearer:** `Authorization: Bearer {token}`

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
- `Cookie: token={amd_token}`
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

## Planned Updates (TODO)

### Add Pre-Built URL Bases to Response

Currently, the `/api/token` endpoint returns:
```json
{
  "token": "...",
  "webserverUrl": "https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP",
  "createdAt": "..."
}
```

**Planned update** - Return additional pre-built URLs for each API type:
```json
{
  "token": "...",
  "webserverUrl": "https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP",
  "xmlrpcUrl": "https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx",
  "restApiBase": "https://providerapi.advancedmd.com/api/api-801/YOURAPP",
  "ehrApiBase": "https://providerapi.advancedmd.com/ehr-api/api-801/YOURAPP",
  "createdAt": "..."
}
```

**Why?** ElevenLabs dynamic variables don't support string manipulation. Pre-built URLs allow direct use:
- `{{amd_xmlrpc_url}}` → for XMLRPC calls (addpatient, etc.)
- `{{amd_ehr_api_base}}/files/documents` → for EHR REST calls
- `{{amd_rest_api_base}}/masterfiles/olsprofiles` → for PM REST calls

### Update ElevenLabs Dynamic Variable Assignments

After implementing the above, update the token tool assignments:

| Variable | Value Path |
|----------|------------|
| `amd_token` | `token` |
| `amd_webserver` | `webserverUrl` |
| `amd_xmlrpc_url` | `xmlrpcUrl` |
| `amd_ehr_api_base` | `ehrApiBase` |
| `amd_rest_api_base` | `restApiBase` |

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
