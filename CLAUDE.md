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

The `/api/token` endpoint returns pre-formatted values optimized for ElevenLabs dynamic variables:

- `token`: Includes "Bearer " prefix → Use directly as `Authorization: {amd_token}`
- `cookieToken`: Includes "token=" prefix → Use directly as `Cookie: {amd_cookie_token}`
- URLs: Exclude "https://" prefix → Use as `https://{amd_rest_api_base}/endpoint`

This is because ElevenLabs doesn't support string concatenation in dynamic variables.

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
│   │   └── patient.go           # Patient model + DOB normalization
│   ├── auth/
│   │   ├── authenticator.go     # 2-step AdvancedMD authentication
│   │   └── token_manager.go     # Background refresh + caching
│   ├── clients/
│   │   ├── redis.go             # Pooled Redis client
│   │   └── advancedmd_xmlrpc.go # XMLRPC client for patient lookup
│   └── http/
│       ├── router.go            # chi router setup
│       ├── handlers.go          # Request handlers
│       └── middleware.go        # Auth, logging, request ID
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

## AdvancedMD API Quirks to Know

1. **Step 1 returns "error"**: The first login step returns `success="0"` with an error code, but this is expected - the webserver URL is still in the response

2. **XML charset issues**: AdvancedMD may return ISO-8859-1 encoded XML, requiring charset handling (see `parseXMLResponse` in auth.go)

3. **Token in Cookie vs Authorization**:
   - XMLRPC APIs use `Cookie: token={token}`
   - REST APIs use `Authorization: Bearer {token}`

4. **URL transformations**: Different API types require transforming the webserver URL by replacing path segments

5. **getdemographic class matters**: Using `class="api"` omits insurance data entirely. Use `class="demographics"` to get `insplanlist` and `carrierlist` in the response

6. **Carrier IDs**: Real carrier IDs are mapped in `internal/domain/patient.go` CarrierMap. Use `lookupcarrier` XMLRPC action to find new carrier IDs (180 carriers across 4 pages)

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
