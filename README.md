# AdvancedMD Token Management Service

A high-performance Go microservice deployed on Vercel that handles AdvancedMD's 2-step authentication flow and caches tokens in Upstash Redis. Designed for integration with ElevenLabs conversational agents.

## Features

- **Fast**: Go runtime with ~50ms cold starts on Vercel
- **Cached**: Tokens stored in Upstash Redis with 23-hour TTL
- **Automated**: Vercel Cron refreshes tokens every 20 hours
- **Fallback**: On-demand token refresh if cache is empty
- **Secure**: API key authentication on all endpoints

## Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│  Vercel Cron    │──────│  /api/cron      │──────│  Upstash Redis  │
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

### 2. Set Up Upstash Redis

1. Go to [Upstash Console](https://console.upstash.com/)
2. Create a new Redis database
3. Copy the REST URL and REST Token

### 3. Configure Environment Variables

In Vercel Dashboard → Settings → Environment Variables, add:

| Variable | Description |
|----------|-------------|
| `ADVANCEDMD_USERNAME` | Your AdvancedMD API username |
| `ADVANCEDMD_PASSWORD` | Your AdvancedMD API password |
| `ADVANCEDMD_OFFICE_KEY` | Your office key (e.g., `991NNN`) |
| `ADVANCEDMD_APP_NAME` | Your registered app name |
| `UPSTASH_REDIS_REST_URL` | Upstash Redis REST URL |
| `UPSTASH_REDIS_REST_TOKEN` | Upstash Redis REST Token |
| `CRON_SECRET` | Random secret for cron endpoint |
| `API_SECRET` | Random secret for token endpoint |

### 4. Deploy to Vercel

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
| `amd_token` | `$.token` |
| `amd_webserver` | `$.webserverUrl` |

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
│       └── redis.go     # Upstash Redis client
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

- All credentials in Vercel Environment Variables (encrypted)
- Cron endpoint protected by `CRON_SECRET`
- Token endpoint protected by `API_SECRET`
- Redis connection uses TLS

## Troubleshooting

### Token endpoint returns 401
- Verify `API_SECRET` is set in Vercel environment variables
- Check the `Authorization` header format: `Bearer YOUR_SECRET`

### Authentication fails
- Verify AdvancedMD credentials are correct
- Check `ADVANCEDMD_OFFICE_KEY` format
- Ensure `ADVANCEDMD_APP_NAME` is registered with AdvancedMD

### Redis connection fails
- Verify Upstash credentials are correct
- Check `UPSTASH_REDIS_REST_URL` format (should be `https://...`)

## License

MIT

## Support

For issues, please open a GitHub issue or contact support.
