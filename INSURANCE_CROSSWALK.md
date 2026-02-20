# Insurance Crosswalk — Spring Hill Location

Source: Abita Insurance List - SpringHill Location rev 9.4.2025
AMD carrier IDs pulled from live system (office 139464) on 2026-02-20.

## Routing Rules

The default is **all 3 doctors** (Bach, Licht, Noel). The rules below are the exceptions.

### Rule 1: NOT ACCEPTED (self-pay only)

| Insurance | Reason |
|-----------|--------|
| Aetna EPO / University of Miami | Out of network |
| AvMed Medicare Advantage (via EMI) | EMI doesn't manage Hernando County |
| Doctors Health Medicare (Vision) | Routine vision only, not medical |
| Florida Blue HMO | EMI doesn't manage Hernando County |
| Florida BlueSelect | Out of network |
| Molina Market Place | No longer accepting (unpaid) |
| Preferred Care Network / Preferred Care Partners | Doesn't manage Hernando County |

### Rule 2: DR. BACH ONLY

| Insurance | Notes |
|-----------|-------|
| Aetna EPO / North Broward Hospital | |
| Cigna Local Plus | Spring Hill location |
| Eye America AAO | No billing, $VISTA only |
| Florida Blue - Steward Healthcare Tier 1 | |
| Humana Medicaid HMO | Requires prior auth/referral |
| Humana Medicare HMO | Requires prior auth/referral |
| Humana Medicare PPO | |
| Humana PPO/POS | No auth required |
| Humana Premier HMO (Access) | No auth required |
| Meritain Health (Aetna) | |
| Molina Medicare | Requires referral from PCP |

**Shortcut: All Humana plans → Bach only.**

### Rule 3: BACH + LICHT ONLY

| Insurance | Notes |
|-----------|-------|
| AvMed (all accepted plans) | Select, Broad Network, Tier B, JH, ESO, MDC, BH South |
| Cigna Medicare Advantage (HMO & PPO) | |
| Oscar Health Plans | ODs cannot see medical for Oscar |
| Tricare Prime | Requires prior auth |
| Tricare Select | No auth required |
| Tricare for Life | Secondary to Medicare |
| United Healthcare Individual Exchange | Requires referral from PCP |

**Shortcut: All Tricare plans → Bach + Licht.**

### Rule 4: ALL 3 DOCTORS (default)

Everything not listed above, including:

- Aetna (Commercial/Medicare PPO, Healthy Kids, Better Health Medicaid, QHP Exchange, Medicare HMO)
- Ambetter (all plans)
- Children's Medical Services
- Cigna (HMO, Open Access, Miami-Dade Public Schools, PPO)
- Community Care Plan / 2020 Medical
- Florida Blue (Medicare PPO, PPO Out of State, PPO Federal Employee)
- Florida Complete Care
- Florida Community Care (Medicaid)
- Imagine Health
- Medicaid (straight)
- Medicare (Part B)
- Miami Children's Health Plan (Medicaid)
- Molina Medicaid
- Multiplan / PHCS
- Simply Medicaid / Healthy Kids
- Staywell Medicare
- SunHealth (discount plan)
- Sunshine Medicaid
- UMR (United Health One)
- United Healthcare (Commercial, AARP Medicare, All Savers, Golden Rule, NHP HMO Access, NHP HMO, Shared Services, Student Resources, Surest/Bind, Global)
- Vivida (Medicaid)
- Wellcare (Medicaid)

---

## AMD Carrier ID Mapping

### Carriers with definitive routing (by carrier ID alone)

These carriers have a single routing rule regardless of plan.

#### NOT ACCEPTED

| Carrier ID | Code | AMD Name | Insurance |
|------------|------|----------|-----------|
| car281648 | DOCT1 | DOCTORS HEALTHCARE PLANS INC | Doctors Health Medicare |
| car40916 | PRE04 | PREFERRED CARE PARTNERS | Preferred Care Network/Partners |
| car301737 | AVMCR | EYE MANAGEMENT INC | AvMed Medicare Advantage via EMI |
| car280750 | FLOR1 | EYE MANAGEMENT INC | Florida Blue HMO via EMI |

#### BACH ONLY → columns: [1513]

| Carrier ID | Code | AMD Name | Insurance |
|------------|------|----------|-----------|
| car303033 | HUM02 | HUMANA MEDICAID | Humana Medicaid HMO |
| car40906 | HUM01 | HUMANA MEDICARE | Humana Medicare HMO/PPO |
| car303062 | HUM PPO | HUMANA PPO POS | Humana PPO/POS |
| car303061 | HUMPHMO | HUMANA PREMIER HMO | Humana Premier HMO (Access) |
| car308175 | HUM03 | HUMANA GOLD PLAN | Humana Gold |
| car308627 | EYEC1 | EYECARE AMERICA AAO | Eye America AAO |
| car301578 | MERI1 | MERITAIN HEALTH | Meritain Health (Aetna) |

#### BACH + LICHT → columns: [1513, 1551]

| Carrier ID | Code | AMD Name | Insurance |
|------------|------|----------|-----------|
| car40890 | AVM01 | AVMED | AvMed (all accepted plans) |
| car302890 | CIGN4 | CIGNA MEDICARE ADVTG HEALTHSPRING | Cigna Medicare Advantage |
| car284233 | OSCA1 | OSCAR INSURANCE COMPANY OF FLORIDA | Oscar Health Plans |
| car284327 | TRI00 | TRICARE EAST | Tricare Prime/Select |
| car40921 | TRI05 | TRICARE FOR LIFE | Tricare for Life |
| car40922 | TRI04 | TRICARE NORTH AND SOUTH REGIONS | Tricare Prime/Select |

#### ALL 3 DOCTORS → columns: [1513, 1551, 1550]

| Carrier ID | Code | AMD Name | PDF Insurance | Notes |
|------------|------|----------|---------------|-------|
| car40887 | AET07 | AETNA | Aetna (Commercial/PPO) | *Has plan-level exceptions (see below) |
| car280636 | AET05 | AETNA BETTER HEALTH | Aetna Better Health Medicaid | |
| car281481 | ABH | AETNA BETTER HEALTH OF FLORIDA | Aetna Better Health Medicaid FL | |
| car302877 | AETN3 | AETNA MEDICARE HMO | Aetna Medicare HMO | |
| car284682 | AMB03 | AMBETTER FROM SUNSHINE HEALTH | Ambetter (all plans) + Sunshine Medicaid | Sunshine is same parent company (Centene) |
| car301345 | CIGN1 | CIGNA HMO | Cigna HMO | *Cigna Local Plus may also use this ID (see below) |
| car40895 | CIG09 | CIGNA PPO | Cigna PPO / Open Access | |
| car301592 | CIGN2 | CIGNA PPO | Cigna PPO (alternate entry) | |
| car307992 | CCP 1 | COMMUNITY CARE PLAN | Community Care Plan / 2020 Medical | |
| car281563 | COMM1 | COMMUNITY CARE PLAN MMCP MCHP | Community Care Plan + Miami Children's Health Plan | MCHP = Miami Children's Health Plan |
| car40897 | FLO01 | FLORIDA BLUE SHIELD | Florida Blue (PPO/Medicare PPO) | *Has plan-level exceptions (see below) |
| car301686 | BLUE1 | BLUE CROSS BLUE SHIELD OF FLORIDA | Florida Blue (alternate entry) | |
| car40899 | FLO03 | FLORIDA MEDICAID | Medicaid (straight) / FL Community Care | Also covers Children's Medical Services, Vivida |
| car40900 | FLO02 | FLORIDA MEDICARE | Medicare (Part B) | |
| car40901 | FRE01 | FREEDOM HEALTH | Florida Complete Care | |
| car40912 | MOL10 | MOLINA HEALTHCARE OF FLORIDA | Molina Medicaid | *Has plan-level exceptions (see below) |
| car301648 | PHCS | PHCS | Multiplan/PHCS | |
| car308086 | SUNHEALT | SUN HEALTH DISCOUNT PLAN | SunHealth | |
| car308142 | IMAG1 | IMAGINE360 HEALTH | Imagine Health | |
| car281218 | EYEQ1 | SIMPLY - EYEQUEST | Simply Medicaid / Healthy Kids | |
| car40907 | ICA01 | ICARE HEALTH OPTIONS TPA | iCare TPA (Children's Medical Services, Vivida, others) | Network/TPA used by multiple Medicaid plans |
| car40923 | UNI20 | UNITED HEALTHCARE | United Healthcare (Commercial) | *Has plan-level exceptions (see below) |
| car302744 | UHC 1 | UHC MEDICARE | United AARP Medicare / Medicare Advantage | |
| car303047 | UNIT9 | UHC SHARED SERVICES | UHC Shared Services | |
| car283950 | UHC STU | UHC STUDENT RESOURCES | UHC Student Resources | |
| car284949 | ALL 1 | ALL SAVERS | United Healthcare All Savers | |
| car284232 | UNIT2 | UNITED HEALTH ONE GOLDEN RULE INSU | UHC Golden Rule | |
| car284838 | UNIT3 | UNITED HEALTHCARE UMR | UMR | |
| car284971 | UNIT5 | UNITED HEALTHCARE GLOBAL | UHC Global | |
| car301501 | BIND1 | SUREST BIND BENEFITS, INC | UHC Surest (formerly Bind) | |
| car40913 | NEI03 | NEIGHBORHOOD HEALTH PARTNERSHIP | UHC NHP HMO / Access | |
| car40925 | WEL04 | WELLCARE HEALTH PLANS | Wellcare Medicaid + Staywell Medicare | Staywell is now part of WellCare/Centene |
| car284306 | ACTI1 | ACTIVE DUTY MILITARY | Active duty military | Not Tricare billing |
| car281245 | AMBE1 | ENVOLVE VISION | Envolve network plans | TPA for Children's Medical Services, Sunshine, etc. |

### Carriers with plan-level ambiguity

These carriers cover multiple plans with DIFFERENT routing rules. The carrier ID alone is not enough — the plan name or additional fields from the patient's demographics are needed. We default to ALL 3 (the most common/permissive rule).

| Carrier ID | Code | AMD Name | Default Rule | Exceptions |
|------------|------|----------|-------------|------------|
| car40887 | AET07 | AETNA | ALL 3 | EPO/North Broward → Bach only; EPO/UM → not accepted |
| car40897 | FLO01 | FLORIDA BLUE SHIELD | ALL 3 | BlueSelect → not accepted; Steward Tier 1 → Bach only |
| car40912 | MOL10 | MOLINA HEALTHCARE OF FLORIDA | ALL 3 | Medicare → Bach only; Marketplace → not accepted |
| car40923 | UNI20 | UNITED HEALTHCARE | ALL 3 | Individual Exchange → Bach + Licht |
| car301345 | CIGN1 | CIGNA HMO | ALL 3 | Cigna Local Plus → Bach only (if filed under this carrier) |

For these, the ElevenLabs agent should ask clarifying questions if the patient mentions one of the exception plans.

### Insurances with no dedicated AMD carrier entry

These plans from the PDF don't have their own carrier entry in AMD. They are likely billed under a parent/generic carrier. All are ALL 3 DOCTORS so routing is unaffected.

| PDF Insurance | Likely AMD Carrier | Reasoning |
|---------------|-------------------|-----------|
| Children's Medical Services | car40899 FLORIDA MEDICAID or car40907 ICARE HEALTH OPTIONS TPA | Medicaid managed care, network is iCare/Envolve |
| Cigna Local Plus | car301345 CIGNA HMO | No separate Cigna Local Plus carrier in AMD |
| Cigna Open Access | car40895 CIGNA PPO | No separate carrier; same routing (all 3) |
| Florida Community Care (ILF Medicaid) | car40899 FLORIDA MEDICAID | State Medicaid program |
| Staywell Medicare | car40925 WELLCARE HEALTH PLANS | Staywell merged into WellCare (Centene) |
| Sunshine Medicaid | car284682 AMBETTER FROM SUNSHINE HEALTH | Same parent company (Centene) |
| Vivida Medicaid | car40899 FLORIDA MEDICAID or car40907 ICARE HEALTH OPTIONS TPA | Small Medicaid plan, network is iCare |

---

## Code-Ready Mapping

```go
// InsuranceRouting maps AMD carrier IDs to allowed scheduler column IDs.
// Default (not listed) = all columns [1513, 1551, 1550]

var NotAcceptedCarriers = map[string]bool{
    "car281648": true, // DOCTORS HEALTHCARE PLANS INC
    "car40916":  true, // PREFERRED CARE PARTNERS
    "car301737": true, // EYE MANAGEMENT INC (AvMed Medicare Adv via EMI)
    "car280750": true, // EYE MANAGEMENT INC (FL Blue HMO via EMI)
}

var BachOnlyCarriers = map[string]bool{
    "car303033": true, // HUMANA MEDICAID
    "car40906":  true, // HUMANA MEDICARE
    "car303062": true, // HUMANA PPO POS
    "car303061": true, // HUMANA PREMIER HMO
    "car308175": true, // HUMANA GOLD PLAN
    "car308627": true, // EYECARE AMERICA AAO
    "car301578": true, // MERITAIN HEALTH
}

var BachLichtCarriers = map[string]bool{
    "car40890":  true, // AVMED
    "car302890": true, // CIGNA MEDICARE ADVTG HEALTHSPRING
    "car284233": true, // OSCAR INSURANCE COMPANY OF FLORIDA
    "car284327": true, // TRICARE EAST
    "car40921":  true, // TRICARE FOR LIFE
    "car40922":  true, // TRICARE NORTH AND SOUTH REGIONS
}

// AllColumns = []string{"1513", "1551", "1550"} // Bach, Licht, Noel
// BachOnly   = []string{"1513"}
// BachLicht  = []string{"1513", "1551"}
```

---

## Implementation Plan

### New Patient Flow

1. Patient tells the ElevenLabs agent their insurance (e.g., "I have Humana Medicare")
2. The LLM normalizes the text and calls **`POST /api/insurance/match`**
   - Input: `{"name": "humana medicare"}`
   - Server does keyword matching against our 260 AMD carrier names
   - Returns: carrier_id, carrier_name, routing rule, allowed providers
3. If **not accepted** → agent tells the patient immediately, offers self-pay
4. If **ambiguous** (e.g., patient says "Aetna" but multiple Aetna plans have different rules) → returns top matches so the agent can ask "Is that Aetna PPO, Aetna Medicare, or Aetna Better Health?"
5. If **matched** → carrier_id is used when calling `addpatient`, and routing rule determines which providers to show for booking

### Existing Patient Flow

1. Patient calls, agent verifies them via `getpatient`
2. We pull demographics via `getdemographic` — response includes `insplanlist.insplan.@carrier` (e.g., `car40906`)
3. Look up carrier ID in our routing map
4. **Return the routing rule alongside the patient verification response** — the agent knows which doctors this patient can see before they ask to book

### Booking (Both Flows)

Add optional `carrier_id` parameter to **`GET /api/scheduler/availability`**:

- Server looks up carrier_id in the routing map
- Intersects allowed columns with existing `AllowedColumns`
- If carrier is **not accepted** → return error with message (e.g., "Humana Gold Plan is not accepted at Spring Hill")
- If carrier **limits providers** → only fetch and return those providers' slots
- If carrier is **all 3** or no carrier_id provided → existing behavior (all providers)

This keeps routing logic server-side — we can update rules without touching ElevenLabs config.

### Insurance Match Endpoint Design

**`POST /api/insurance/match`**

Request:
```json
{"name": "humana medicare"}
```

Response (single match):
```json
{
  "matched": true,
  "carrier_id": "car40906",
  "carrier_name": "HUMANA MEDICARE",
  "routing": "bach_only",
  "allowed_columns": ["1513"],
  "allowed_providers": ["Dr. Bach"]
}
```

Response (not accepted):
```json
{
  "matched": true,
  "carrier_id": "car40916",
  "carrier_name": "PREFERRED CARE PARTNERS",
  "routing": "not_accepted",
  "message": "This insurance is not accepted at Spring Hill. Preferred Care Partners does not manage Hernando County."
}
```

Response (ambiguous — multiple matches):
```json
{
  "matched": false,
  "candidates": [
    {"carrier_id": "car40887", "carrier_name": "AETNA", "routing": "all_three"},
    {"carrier_id": "car280636", "carrier_name": "AETNA BETTER HEALTH", "routing": "all_three"},
    {"carrier_id": "car302877", "carrier_name": "AETNA MEDICARE HMO", "routing": "all_three"}
  ],
  "message": "Multiple Aetna plans found. Please ask the patient which plan they have."
}
```

Response (no match):
```json
{
  "matched": false,
  "candidates": [],
  "message": "Insurance not found in our system. The patient may need to provide more details or may be out of network."
}
```

### Matching Logic

1. **Normalize** — lowercase, trim whitespace
2. **Exact match** — check if input matches a carrier name exactly
3. **Keyword match** — split input into words, find carriers whose name contains all keywords (e.g., "humana medicare" matches "HUMANA MEDICARE")
4. **Partial match** — find carriers whose name contains any keyword, ranked by number of keyword hits
5. **Carrier family shortcuts** — common names map to carrier families:
   - "humana" → all Humana carriers
   - "tricare" → all Tricare carriers
   - "cigna" → all Cigna carriers
   - "aetna" → all Aetna carriers
   - "united" / "uhc" → all UHC carriers
   - "florida blue" / "blue cross" → Florida Blue carriers
   - "molina" → all Molina carriers
   - "oscar" → Oscar
   - "avmed" → AvMed
   - "ambetter" → Ambetter
   - "medicaid" → Florida Medicaid
   - "medicare" → Florida Medicare

### Edge Cases

- **Patient doesn't know their insurance** → agent can still book, warns office will verify at check-in
- **Existing patient's insurance changed** → agent should confirm "Do you still have [carrier name]?" during verification
- **Patient has plan-level ambiguity** (e.g., carrier is AETNA but could be EPO/UM which is not accepted) → for the 5 ambiguous carriers, the agent asks a clarifying question about their specific plan type
- **Insurance not in AMD at all** → likely out-of-network or very rare plan; agent offers self-pay or asks patient to call the office
