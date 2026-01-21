# AdvancedMD API Documentation

Documentation of AdvancedMD APIs used for ElevenLabs agent integration.

---

## Table of Contents

- [Authentication](#authentication)
- [XMLRPC API](#xmlrpc-api)
  - [lookuppatient](#lookuppatient)
  - [getschedulersetup](#getschedulersetup)
  - [getappttypes](#getappttypes)
- [REST API](#rest-api)
  - [Columns Openings](#columns-openings)
  - [Book Appointment](#book-appointment)
- [Middleware Solutions](#middleware-solutions)
  - [/api/verify-patient](#apiverify-patient)

---

## Authentication

AdvancedMD uses a 2-step authentication flow. See the main README for details.

**Key Points:**
- Tokens are valid for ~24 hours
- Use `Cookie: token={token}` header for XMLRPC APIs
- Use `Authorization: Bearer {token}` header for REST APIs

---

## XMLRPC API

The XMLRPC API is used for core patient operations.

### Endpoint

```
POST https://{xmlrpcUrl}
```

### Required Headers

| Header | Value | Description |
|--------|-------|-------------|
| `Cookie` | `token={token}` | Session token from authentication |
| `Content-Type` | `application/json` | Request format |
| `Accept` | `application/json` | Response format (required for JSON response) |

---

### lookuppatient

Search for patients by name.

#### Minimum Required Body

```json
{
    "ppmdmsg": {
        "@action": "lookuppatient",
        "@class": "api",
        "@name": "Smith"
    }
}
```

#### All Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `@action` | Yes | Action name | `"lookuppatient"` |
| `@class` | Yes | API class | `"api"` |
| `@name` | Yes | Search string (surname or "Surname,Firstname") | `"Smith"` or `"Smith,John"` |
| `@msgtime` | No | Timestamp | `"1/21/2026 2:30:00 PM"` |
| `@exactmatch` | No | `-1` for exact match only | `"-1"` |
| `@page` | No | Page number for pagination | `"1"` |
| `@nocookie` | No | Cookie handling flag | `"0"` |

#### Example Request

```bash
curl -X POST "https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx" \
  -H "Cookie: token=YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{
    "ppmdmsg": {
        "@action": "lookuppatient",
        "@class": "api",
        "@name": "Smith"
    }
}'
```

#### Example Response

```json
{
  "PPMDResults": {
    "Results": {
      "patientlist": {
        "@itemcount": "2",
        "@page": "1",
        "@pagecount": "1",
        "patient": [
          {
            "@id": "pat45",
            "@name": "SMITH,LINDA F C",
            "@chart": "MCCLO000",
            "@ssn": "329-42-9086",
            "@gender": "F",
            "@dob": "09/20/1976",
            "address": {
              "@zip": "85745",
              "@city": "TUCSON",
              "@state": "AZ",
              "@address1": "# 32",
              "@address2": "9679 N CAMINO PIMERIA ALTA"
            },
            "contactinfo": {
              "@homephone": "(520) 921-6692"
            }
          },
          {
            "@id": "pat25",
            "@name": "SMITH,SUSAN",
            "@chart": "GUZRE000",
            "@ssn": "066-38-2602",
            "@gender": "F",
            "@dob": "02/26/1968",
            "address": {
              "@zip": "85653",
              "@city": "MARANA",
              "@state": "AZ"
            },
            "contactinfo": {
              "@homephone": "(520) 436-0101"
            }
          }
        ]
      }
    }
  }
}
```

#### Response Fields

| Field | Description |
|-------|-------------|
| `@id` | **Patient ID** - Use this for subsequent API calls |
| `@name` | Full name in "LASTNAME,FIRSTNAME" format |
| `@dob` | Date of birth in MM/DD/YYYY format |
| `@gender` | M or F |
| `@chart` | Chart number |
| `@ssn` | Social security number (masked in some environments) |
| `contactinfo.@homephone` | Home phone number |
| `address.*` | Address fields |

#### Notes

- **Single result**: When only one patient matches, AMD returns `patient` as an object, not an array
- **Pagination**: Use `@page` parameter and check `@pagecount` in response
- **Partial match**: By default, searches are partial matches. Use `@exactmatch="-1"` for exact matches only

---

### getschedulersetup

Returns the practice's scheduler configuration including columns (resources), profiles (providers), and facilities (locations).

#### Minimum Required Body

```json
{
    "ppmdmsg": {
        "@action": "getschedulersetup",
        "@class": "masterfiles",
        "@msgtime": "1/21/2026 12:00:00 PM"
    }
}
```

#### Response Contains

- **columnlist**: Scheduling columns (resources/doctors)
- **profilelist**: Provider profiles
- **facilitylist**: Locations/facilities
- **pagelist**: Scheduler page configuration

#### Key Response Fields

**Columns:**
| Field | Description |
|-------|-------------|
| `@id` | Column ID (e.g., "col2") - use numeric part for API calls |
| `@name` | Display name (e.g., "JONES") |
| `@profile` | Associated profile ID |
| `@facility` | Associated facility ID |
| `columnsetting.@start` | Start time (e.g., "08:30") |
| `columnsetting.@end` | End time (e.g., "17:00") |
| `columnsetting.@interval` | Time slot interval in minutes |
| `columnsetting.@workweek` | 7-char string for Sun-Sat (1=working, 0=off) |

**Profiles:**
| Field | Description |
|-------|-------------|
| `@id` | Profile ID (e.g., "prof3") - use numeric part for API calls |
| `@code` | Short code |
| `@name` | Full provider name |

---

### getappttypes

Returns all appointment types configured in the practice.

#### Minimum Required Body

```json
{
    "ppmdmsg": {
        "@action": "getappttypes",
        "@class": "masterfiles",
        "@msgtime": "1/21/2026 12:00:00 PM",
        "appttype": ""
    }
}
```

#### Key Response Fields

| Field | Description |
|-------|-------------|
| `@id` | Appointment type ID (e.g., "ap_type19") - use numeric part |
| `@name` | Type name (e.g., "ANNUAL EXAM") |
| `@hide` | "yes" = deleted/hidden, "no" = active |
| `appttypesettings.@duration` | Default duration in minutes |
| `appttypesettings.@color` | Calendar color |

---

## REST API

The REST API is used for scheduling operations.

### Endpoint

```
POST https://{restApiBase}/scheduler/...
```

### Required Headers

| Header | Value | Description |
|--------|-------|-------------|
| `Authorization` | `Bearer {token}` | Session token from authentication |
| `Content-Type` | `application/json` | Request format |
| `Accept` | `application/json` | Response format |

---

### Columns Openings

Search for available appointment slots.

#### Endpoint

```
POST https://{restApiBase}/scheduler/Columns/openings
```

#### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `startdate` | datetime | Search start date (`"2026-01-22T08:00:00"`) |
| `appointmenttimerange` | datetime | Search end date (`"2026-01-30T17:00:00"`) |
| `daysofweek` | string | 7-char string for Sun-Sat (`"1111111"` = all days) |
| `duration` | integer | Number of time slots needed (not minutes!) |
| `profileids` | array | Profile IDs to search (`[3, 4]`) |
| `columnids` | array | Column IDs to search (`[2, 3]`) |

#### Optional Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `forward` | boolean | Search forward from startdate |
| `includeopenslots` | boolean | Include open slots in response |
| `observemaxapptsperslot` | boolean | Respect max appointments per slot |

#### Understanding Duration (Slots vs Minutes)

`duration` = **number of time slots**, not minutes!

Based on the column's interval setting:
- Column interval = 15 min, 30-min appointment → `duration: 2`
- Column interval = 15 min, 15-min appointment → `duration: 1`
- Column interval = 30 min, 30-min appointment → `duration: 1`

#### Location Filtering

**You cannot filter by `facilityid` directly.** Instead, filter by location using `columnids`:

1. Each column is assigned to a facility in your scheduler setup
2. Group columns by their facility
3. Pass the appropriate `columnids` array for the desired location

**Example Location Mapping:**
```
Abita Springs (facilityid: 101)
  → columnids: [2, 7], profileids: [3, 6]

Covington (facilityid: 102)
  → columnids: [3], profileids: [4]

Mandeville (facilityid: 103)
  → columnids: [5], profileids: [5]
```

#### Example Request

```bash
curl -X POST "https://{restApiBase}/scheduler/Columns/openings" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "startdate": "2026-01-22T08:00:00",
    "appointmenttimerange": "2026-01-30T17:00:00",
    "daysofweek": "1111111",
    "profileids": [3],
    "columnids": [2],
    "duration": 1,
    "forward": true,
    "includeopenslots": true,
    "observemaxapptsperslot": true
  }'
```

#### Example Response

```json
{
  "startdate": "2026-01-22T00:00:00",
  "lastsearchdate": "2026-01-22T00:00:00",
  "hasopenslot": true,
  "openslotdate": "2026-01-22T00:00:00",
  "columns": [
    {
      "id": 2,
      "heading": "JONES",
      "starttime": "08:30",
      "endtime": "17:00",
      "timeincrement": 30,
      "workweek": "0111100",
      "maxapptsperslot": 0,
      "hasopenslot": true,
      "openslotdate": "2026-01-22T08:30:00",
      "facilityid": null,
      "profileid": 3
    }
  ]
}
```

#### Response Fields

| Field | Description |
|-------|-------------|
| `hasopenslot` | Whether any openings were found |
| `openslotdate` | Date of first available opening |
| `columns[].id` | Column ID (use for booking) |
| `columns[].heading` | Doctor/resource name |
| `columns[].profileid` | Profile ID (use for booking) |
| `columns[].hasopenslot` | Whether this column has openings |
| `columns[].openslotdate` | First available slot for this column |
| `columns[].timeincrement` | Minutes per slot |

---

### Book Appointment

Create a new appointment.

#### Endpoint

```
POST https://{restApiBase}/scheduler/Appointments
```

#### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `patientid` | integer | Patient ID from verify_patient |
| `columnid` | integer | Column ID from Columns Openings |
| `startdatetime` | datetime | Appointment date/time (`"2026-01-22T10:30"`) |
| `duration` | integer | Duration **in minutes** (not slots!) |
| `profileid` | integer | Profile ID from Columns Openings |
| `type` | array | Appointment type: `[{"id": 19}]` |

#### Optional Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `facilityid` | integer | Facility/location ID |
| `color` | string | Calendar color |
| `instruction` | array | Patient instructions |

#### Duration: Slots vs Minutes

| API | Duration Unit |
|-----|---------------|
| Columns Openings | **Slots** (1, 2, 3...) |
| Book Appointment | **Minutes** (15, 30, 60...) |

#### Minimum Required Body

```json
{
  "patientid": 5984942,
  "columnid": 2,
  "startdatetime": "2026-01-22T10:30",
  "duration": 30,
  "profileid": 3,
  "type": [{"id": 19}]
}
```

#### Example Request

```bash
curl -X POST "https://{restApiBase}/scheduler/Appointments" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "patientid": 5984942,
    "columnid": 2,
    "startdatetime": "2026-01-22T10:30",
    "duration": 30,
    "profileid": 3,
    "type": [{"id": 19}]
  }'
```

---

## Scheduling Workflow

```
1. verify_patient
   └─→ Get patient_id

2. Columns Openings
   └─→ Get available slots (columnid, profileid, openslotdate)

3. User picks a time

4. Book Appointment
   └─→ Create appointment using:
       - patientid (from step 1)
       - columnid, profileid (from step 2)
       - startdatetime (from user selection)
       - duration (in minutes)
       - type (appointment type id)
```

---

## Practice Configuration Template

Use this template to hardcode your practice's configuration in ElevenLabs prompts:

```markdown
## Locations
| Location | Facility ID | Column IDs | Profile IDs |
|----------|-------------|------------|-------------|
| Abita Springs | 101 | [2, 7] | [3, 6] |
| Covington | 102 | [3] | [4] |
| Mandeville | 103 | [5] | [5] |

## Doctors
| Doctor | Profile ID | Column ID | Location |
|--------|------------|-----------|----------|
| Dr. Jones | 3 | 2 | Abita Springs |
| Dr. Adams | 6 | 7 | Abita Springs |
| Dr. Smith | 4 | 3 | Covington |
| Dr. Richey | 5 | 5 | Mandeville |

## Appointment Types
| Type | Type ID | Duration (min) | Slots (15-min interval) |
|------|---------|----------------|-------------------------|
| New Patient | 12 | 15 | 1 |
| Established Patient | 7 | 15 | 1 |
| Annual Exam | 19 | 15 | 1 |
| Consultation | 9 | 30 | 2 |
| Follow Up | 13 | 15 | 1 |
| Procedure | 10 | 60 | 4 |
```

---

## Middleware Solutions

These endpoints simplify AdvancedMD integration for ElevenLabs agents.

### /api/verify-patient

**Purpose:** Simplified patient verification that handles token management, API calls, and result filtering internally.

#### Why Use This?

ElevenLabs agents have limitations:
- Can't do complex JSON filtering
- Can't store intermediate state
- Can't construct Cookie headers easily

This endpoint solves all of that by:
1. Managing tokens internally
2. Calling the AMD lookuppatient API
3. Filtering results by DOB
4. Returning a simple, actionable response

#### Request

```
POST /api/verify-patient
Authorization: Bearer {API_SECRET}
Content-Type: application/json
```

#### Request Body

```json
{
  "lastName": "Smith",
  "dob": "09/20/1976",
  "firstName": "Linda"    // optional, for disambiguation
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `lastName` | Yes | Patient's last name (partial match) |
| `dob` | Yes | Date of birth (multiple formats accepted) |
| `firstName` | No | First name (only needed if multiple matches) |

#### Accepted DOB Formats

The endpoint normalizes dates automatically:
- `09/20/1976` (MM/DD/YYYY)
- `1976-09-20` (ISO)
- `9/20/1976` (M/D/YYYY)
- `September 20 1976`
- `September 20, 1976`
- `Sep 20 1976`

#### Response Scenarios

**1. Single Match (Verified)**

```json
{
  "status": "verified",
  "patientId": "45",
  "name": "SMITH,LINDA F C",
  "dob": "09/20/1976",
  "phone": "(520) 921-6692"
}
```

**2. No Match Found**

```json
{
  "status": "not_found",
  "message": "No patient found with that last name and date of birth"
}
```

**3. Multiple Matches (Need First Name)**

```json
{
  "status": "multiple_matches",
  "message": "Found 2 patients with that last name and DOB. Please provide first name.",
  "matches": [
    { "firstName": "LINDA F C" },
    { "firstName": "LISA" }
  ]
}
```

**4. Error**

```json
{
  "status": "error",
  "message": "Error description"
}
```

#### ElevenLabs Integration

**1. Create a Server Tool:**

| Field | Value |
|-------|-------|
| Name | `verify_patient` |
| Description | Verifies a patient in the system by last name and date of birth |
| Method | POST |
| URL | `https://advancedmd-token-management.vercel.app/api/verify-patient` |

**2. Headers:**
- `Authorization`: `Bearer {API_SECRET}` (as a secret)
- `Content-Type`: `application/json`

**3. Body Parameters:**
- `lastName` (string, required)
- `dob` (string, required)
- `firstName` (string, optional)

**4. Dynamic Variable Mapping:**

| Variable | JSON Path | Use For |
|----------|-----------|---------|
| `patient_id` | `patientId` | Subsequent API calls (numeric only, no "pat" prefix) |
| `patient_name` | `name` | Confirming with patient |
| `verification_status` | `status` | Flow control |

**5. Agent Flow:**

```
Agent: "What's your last name?"
User: "Smith"

Agent: "And your date of birth?"
User: "September 20th 1976"

→ Calls verify_patient { lastName: "Smith", dob: "September 20th 1976" }

Response: { "status": "verified", "patientId": "pat45", "name": "SMITH,LINDA F C", ... }

Agent: "Thank you Linda, I found your record. How can I help you today?"
```

#### Example cURL

```bash
# Successful verification
curl -X POST "https://advancedmd-token-management.vercel.app/api/verify-patient" \
  -H "Authorization: Bearer YOUR_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"lastName": "Smith", "dob": "09/20/1976"}'

# With first name (for disambiguation)
curl -X POST "https://advancedmd-token-management.vercel.app/api/verify-patient" \
  -H "Authorization: Bearer YOUR_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"lastName": "Smith", "dob": "09/20/1976", "firstName": "Linda"}'
```
