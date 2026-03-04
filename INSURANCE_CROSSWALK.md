# Insurance Crosswalk — Spring Hill Location

Source: Abita Insurance List - SpringHill Location rev 9.4.2025
AMD carrier IDs pulled from live system (office 139464) on 2026-02-20.

## How It Works

The LLM has a fixed list of insurance names in its TOOLS prompt. When a patient says their insurance, the LLM picks the matching name and sends it as a string. The middleware maps name → carrier ID + routing rule.

- **Existing patient**: `verify-patient` pulls carrier ID from AMD demographics → middleware looks up routing → returns allowed providers in the response
- **New patient**: LLM sends insurance name → middleware maps to carrier ID (for `addpatient`) + routing (for scheduling)
- **Scheduling**: `carrier_id` parameter on availability endpoint → middleware filters columns by routing rule

## Routing Rules

4 tiers. Default is **all 3 doctors** (Bach, Licht, Noel).

| Rule | Columns | Providers |
|------|---------|-----------|
| **Not Accepted** | none | Self-pay only |
| **Bach Only** | 1513 | Dr. Bach |
| **Bach + Licht** | 1513, 1551 | Dr. Bach, Dr. Licht |
| **All 3** (default) | 1513, 1551, 1550 | Dr. Bach, Dr. Licht, Dr. Noel |

---

## Insurance Name Map

This is the complete list of names the LLM can send. Each maps to a carrier ID and routing rule.

### NOT ACCEPTED

These insurances are not accepted at Spring Hill. The agent should tell the patient immediately and offer self-pay.

| Name | Carrier ID | Reason |
|------|-----------|--------|
| Aetna EPO University of Miami | car40887 | Out of network |
| AvMed Medicare Advantage | car301737 | EMI doesn't manage Hernando County |
| Doctors Health Medicare | car40907 (iCare) | Routine vision only, not medical |
| Florida Blue HMO | car280750 | EMI doesn't manage Hernando County |
| Florida BlueSelect | car40897 | Out of network |
| Molina Marketplace | car40912 | No longer accepting |
| Preferred Care Partners | car40916 | Doesn't manage Hernando County |

### BACH ONLY

| Name | Carrier ID |
|------|-----------|
| Aetna EPO North Broward | car40887 |
| Cigna Local Plus | car301345 |
| Eye America AAO | car308627 |
| Florida Blue Steward Tier 1 | car40897 |
| Humana Gold | car308175 |
| Humana Medicaid | car303033 |
| Humana Medicare | car40906 |
| Humana PPO | car303062 |
| Humana Premier HMO | car303061 |
| Meritain Health | car301578 |
| Molina Medicare | car40912 |

**Shortcut: All Humana plans → Bach only.**

### BACH + LICHT

| Name | Carrier ID |
|------|-----------|
| AvMed | car40890 |
| Cigna Medicare Advantage | car302890 |
| Oscar Health | car284233 |
| Tricare Prime | car284327 |
| Tricare Select | car40922 |
| Tricare for Life | car40921 |
| United Healthcare Individual Exchange | car40923 |

**Shortcut: All Tricare plans → Bach + Licht.**

### ALL 3 DOCTORS (default)

| Name | Carrier ID |
|------|-----------|
| Aetna | car40887 |
| Aetna Better Health | car40907 (iCare) |
| Aetna Better Health of Florida | car40907 (iCare) |
| Aetna Healthy Kids | car40907 (iCare) |
| Aetna Medicare HMO | car40907 (iCare) |
| Ambetter | car284682 |
| Cigna HMO | car301345 |
| Cigna PPO | car40895 |
| Community Care Plan | car40907 (iCare) |
| Envolve Vision | car281245 |
| Florida Blue | car40897 |
| Florida Community Care | car40907 (iCare) |
| Florida Complete Care | car40907 (iCare) |
| Florida Medicaid | car40899 |
| Florida Medicare | car40900 |
| Imagine Health | car308142 |
| Molina Medicaid | car40912 |
| Multiplan PHCS | car301648 |
| Miami Childrens Health Plan | car40907 (iCare) |
| Simply Medicaid | car40907 (iCare) |
| SunHealth | car308086 |
| United Healthcare | car40923 |
| United Healthcare AARP Medicare | car302744 |
| United Healthcare All Savers | car284949 |
| United Healthcare Global | car284971 |
| United Healthcare Golden Rule | car284232 |
| United Healthcare NHP | car40913 |
| United Healthcare Shared Services | car303047 |
| United Healthcare Student Resources | car283950 |
| United Healthcare Surest | car301501 |
| UMR | car284838 |
| Vivida | car40907 (iCare) |
| Wellcare | car40925 |

---

## iCare Network → ICARE HEALTH OPTIONS TPA (car40907)

Many Medicaid/Medicare managed care plans at Spring Hill bill through the **iCare** network. For AMD billing purposes, all iCare plans use a single carrier: **ICARE HEALTH OPTIONS TPA** (carrier code `ICA01`, carrier ID `car40907`).

| Plan Name | Original Carrier ID | Now Uses |
|-----------|-------------------|----------|
| Aetna Better Health | car280636 | car40907 |
| Aetna Better Health of Florida | car281481 | car40907 |
| Aetna Healthy Kids | *(new)* | car40907 |
| Aetna Medicare HMO | car302877 | car40907 |
| Community Care Plan | car307992 | car40907 |
| Doctors Health Medicare | car281648 | car40907 |
| Florida Community Care | *(new)* | car40907 |
| Florida Complete Care | car40901 | car40907 |
| Miami Childrens Health Plan | *(new)* | car40907 |
| Simply Medicaid | car281218 | car40907 |
| Vivida | *(new)* | car40907 |

**Note:** Molina Medicaid also uses the iCare network per the insurance list but retains its own carrier ID (`car40912`) in AMD.

---

## Shared Carrier IDs (Plan-Level Routing)

These carrier IDs appear in multiple routing tiers. The **name** determines the routing, not the carrier ID alone. This matters for existing patients where we only have the carrier ID from demographics — we default to the most permissive rule (All 3).

| Carrier ID | Default Routing | Exception Names |
|-----------|----------------|-----------------|
| car40887 (AETNA) | All 3 | "Aetna EPO North Broward" → Bach only; "Aetna EPO University of Miami" → Not accepted |
| car40897 (FL BLUE) | All 3 | "Florida Blue Steward Tier 1" → Bach only; "Florida BlueSelect" → Not accepted |
| car40912 (MOLINA) | All 3 | "Molina Medicare" → Bach only; "Molina Marketplace" → Not accepted |
| car40923 (UHC) | All 3 | "United Healthcare Individual Exchange" → Bach + Licht |
| car301345 (CIGNA HMO) | All 3 | "Cigna Local Plus" → Bach only |

**For existing patients**: When we get a carrier ID from demographics and it's one of these 5 ambiguous carriers, we return the default routing (All 3) along with a flag so the agent can ask a clarifying question about the specific plan.

---

## Implementation

### 1. Domain Layer (`internal/domain/insurance.go`)

```go
type RoutingRule string

const (
    RoutingNotAccepted RoutingRule = "not_accepted"
    RoutingBachOnly    RoutingRule = "bach_only"
    RoutingBachLicht   RoutingRule = "bach_licht"
    RoutingAll         RoutingRule = "all_three"
)

type InsuranceEntry struct {
    CarrierID string
    Routing   RoutingRule
}

// InsuranceNameMap maps LLM-provided insurance names to carrier ID + routing.
// Keys are normalized (lowercase, no punctuation) via NormalizeForLookup.
var InsuranceNameMap = map[string]InsuranceEntry{
    // NOT ACCEPTED
    "aetna epo university of miami": {CarrierID: "car40887", Routing: RoutingNotAccepted},
    "avmed medicare advantage":      {CarrierID: "car301737", Routing: RoutingNotAccepted},
    "doctors health medicare":       {CarrierID: "car40907", Routing: RoutingNotAccepted},
    "florida blue hmo":              {CarrierID: "car280750", Routing: RoutingNotAccepted},
    "florida blueselect":            {CarrierID: "car40897", Routing: RoutingNotAccepted},
    "molina marketplace":            {CarrierID: "car40912", Routing: RoutingNotAccepted},
    "preferred care partners":       {CarrierID: "car40916", Routing: RoutingNotAccepted},

    // BACH ONLY
    "aetna epo north broward":       {CarrierID: "car40887", Routing: RoutingBachOnly},
    "cigna local plus":              {CarrierID: "car301345", Routing: RoutingBachOnly},
    "eye america aao":               {CarrierID: "car308627", Routing: RoutingBachOnly},
    "florida blue steward tier 1":   {CarrierID: "car40897", Routing: RoutingBachOnly},
    "humana gold":                   {CarrierID: "car308175", Routing: RoutingBachOnly},
    "humana medicaid":               {CarrierID: "car303033", Routing: RoutingBachOnly},
    "humana medicare":               {CarrierID: "car40906", Routing: RoutingBachOnly},
    "humana ppo":                    {CarrierID: "car303062", Routing: RoutingBachOnly},
    "humana premier hmo":            {CarrierID: "car303061", Routing: RoutingBachOnly},
    "meritain health":               {CarrierID: "car301578", Routing: RoutingBachOnly},
    "molina medicare":               {CarrierID: "car40912", Routing: RoutingBachOnly},

    // BACH + LICHT
    "avmed":                                {CarrierID: "car40890", Routing: RoutingBachLicht},
    "cigna medicare advantage":             {CarrierID: "car302890", Routing: RoutingBachLicht},
    "oscar health":                         {CarrierID: "car284233", Routing: RoutingBachLicht},
    "tricare prime":                        {CarrierID: "car284327", Routing: RoutingBachLicht},
    "tricare select":                       {CarrierID: "car40922", Routing: RoutingBachLicht},
    "tricare for life":                     {CarrierID: "car40921", Routing: RoutingBachLicht},
    "united healthcare individual exchange": {CarrierID: "car40923", Routing: RoutingBachLicht},

    // ALL 3
    "aetna":                              {CarrierID: "car40887", Routing: RoutingAll},
    "aetna better health":                {CarrierID: "car40907", Routing: RoutingAll},
    "aetna better health of florida":     {CarrierID: "car40907", Routing: RoutingAll},
    "aetna healthy kids":                 {CarrierID: "car40907", Routing: RoutingAll},
    "aetna medicare hmo":                 {CarrierID: "car40907", Routing: RoutingAll},
    "ambetter":                           {CarrierID: "car284682", Routing: RoutingAll},
    "cigna hmo":                          {CarrierID: "car301345", Routing: RoutingAll},
    "cigna ppo":                          {CarrierID: "car40895", Routing: RoutingAll},
    "community care plan":                {CarrierID: "car40907", Routing: RoutingAll},
    "envolve vision":                     {CarrierID: "car281245", Routing: RoutingAll},
    "florida blue":                       {CarrierID: "car40897", Routing: RoutingAll},
    "florida community care":             {CarrierID: "car40907", Routing: RoutingAll},
    "florida complete care":              {CarrierID: "car40907", Routing: RoutingAll},
    "florida medicaid":                   {CarrierID: "car40899", Routing: RoutingAll},
    "florida medicare":                   {CarrierID: "car40900", Routing: RoutingAll},
    "imagine health":                     {CarrierID: "car308142", Routing: RoutingAll},
    "molina medicaid":                    {CarrierID: "car40912", Routing: RoutingAll},
    "multiplan phcs":                     {CarrierID: "car301648", Routing: RoutingAll},
    "miami childrens health plan":        {CarrierID: "car40907", Routing: RoutingAll},
    "simply medicaid":                    {CarrierID: "car40907", Routing: RoutingAll},
    "sunhealth":                          {CarrierID: "car308086", Routing: RoutingAll},
    "united healthcare":                  {CarrierID: "car40923", Routing: RoutingAll},
    "united healthcare aarp medicare":    {CarrierID: "car302744", Routing: RoutingAll},
    "united healthcare all savers":       {CarrierID: "car284949", Routing: RoutingAll},
    "united healthcare global":           {CarrierID: "car284971", Routing: RoutingAll},
    "united healthcare golden rule":      {CarrierID: "car284232", Routing: RoutingAll},
    "united healthcare nhp":              {CarrierID: "car40913", Routing: RoutingAll},
    "united healthcare shared services":  {CarrierID: "car303047", Routing: RoutingAll},
    "united healthcare student resources": {CarrierID: "car283950", Routing: RoutingAll},
    "united healthcare surest":           {CarrierID: "car301501", Routing: RoutingAll},
    "umr":                                {CarrierID: "car284838", Routing: RoutingAll},
    "vivida":                             {CarrierID: "car40907", Routing: RoutingAll},
    "wellcare":                           {CarrierID: "car40925", Routing: RoutingAll},
}

// CarrierRoutingMap maps AMD carrier IDs to routing rules.
// Used for existing patients where we get the carrier ID from demographics.
// For the 5 ambiguous carriers, defaults to the most permissive rule.
var CarrierRoutingMap = map[string]RoutingRule{
    // NOT ACCEPTED
    "car281648": RoutingNotAccepted, // DOCTORS HEALTHCARE PLANS INC (legacy, pre-iCare migration)
    "car40916":  RoutingNotAccepted, // PREFERRED CARE PARTNERS
    "car301737": RoutingNotAccepted, // EYE MANAGEMENT INC (AvMed Medicare via EMI)
    "car280750": RoutingNotAccepted, // EYE MANAGEMENT INC (FL Blue HMO via EMI)
    // BACH ONLY
    "car303033": RoutingBachOnly, // HUMANA MEDICAID
    "car40906":  RoutingBachOnly, // HUMANA MEDICARE
    "car303062": RoutingBachOnly, // HUMANA PPO POS
    "car303061": RoutingBachOnly, // HUMANA PREMIER HMO
    "car308175": RoutingBachOnly, // HUMANA GOLD PLAN
    "car308627": RoutingBachOnly, // EYECARE AMERICA AAO
    "car301578": RoutingBachOnly, // MERITAIN HEALTH
    // BACH + LICHT
    "car40890":  RoutingBachLicht, // AVMED
    "car302890": RoutingBachLicht, // CIGNA MEDICARE ADVTG HEALTHSPRING
    "car284233": RoutingBachLicht, // OSCAR INSURANCE COMPANY OF FLORIDA
    "car284327": RoutingBachLicht, // TRICARE EAST
    "car40921":  RoutingBachLicht, // TRICARE FOR LIFE
    "car40922":  RoutingBachLicht, // TRICARE NORTH AND SOUTH REGIONS
    // ALL 3 — only listing carriers that have a unique ID (not shared)
    // Everything else defaults to RoutingAll
}

// AmbiguousCarriers are carrier IDs that span multiple routing tiers.
// When we get these from demographics, we default to All 3 but flag it.
var AmbiguousCarriers = map[string]bool{
    "car40887":  true, // AETNA
    "car40897":  true, // FLORIDA BLUE SHIELD
    "car40912":  true, // MOLINA HEALTHCARE OF FLORIDA
    "car40923":  true, // UNITED HEALTHCARE
    "car301345": true, // CIGNA HMO
}

// ProvidersForRouting returns the display names for a routing rule.
func ProvidersForRouting(rule RoutingRule) []string {
    switch rule {
    case RoutingNotAccepted:
        return nil
    case RoutingBachOnly:
        return []string{"Dr. Bach"}
    case RoutingBachLicht:
        return []string{"Dr. Bach", "Dr. Licht"}
    default:
        return []string{"Dr. Bach", "Dr. Licht", "Dr. Noel"}
    }
}
```

### 2. Existing Patient Flow — Modify `verify-patient` response

Currently returns `insuranceCarrier` (the carrier name string from demographics). Add:

```json
{
  "status": "verified",
  "patientId": "12345",
  "name": "SMITH,JOHN",
  "dob": "01/15/1980",
  "phone": "(352)555-1234",
  "insuranceCarrier": "HUMANA MEDICARE",
  "insuranceCarrierId": "car40906",
  "routing": "bach_only",
  "allowedProviders": ["Dr. Bach"],
  "routingAmbiguous": false
}
```

If the carrier ID is one of the 5 ambiguous carriers:
```json
{
  "insuranceCarrier": "AETNA",
  "insuranceCarrierId": "car40887",
  "routing": "all_three",
  "allowedProviders": ["Dr. Bach", "Dr. Licht", "Dr. Noel"],
  "routingAmbiguous": true
}
```

### 3. New Patient Flow — Modify `add-patient` request

Replace the current `carrierId` field (which uses the old generic CarrierMap) with the insurance name from the LLM:

```json
{
  "firstName": "John",
  "lastName": "Smith",
  "dob": "01/15/1980",
  "insurance": "Humana Medicare",
  "subscriberName": "John Smith",
  "subscriberNum": "H12345678"
}
```

Middleware looks up "Humana Medicare" → gets `car40906` + `bach_only`.

### 4. Scheduling — No changes needed

The availability endpoint already supports a `provider` filter. After verify-patient or add-patient returns the allowed providers, the LLM knows which doctors to request when calling availability. No `carrierId` param needed on the scheduling side.

---

## Edge Cases

- **Patient doesn't know insurance** → agent books without filtering, office verifies at check-in
- **Insurance not in the list** → agent tells patient it may not be accepted, offers self-pay or suggests calling the office
- **Existing patient has ambiguous carrier** → agent asks clarifying question about plan type (e.g., "I see you have Aetna — is that a regular PPO, an EPO, or a Medicare plan?")
- **Existing patient's carrier ID not in CarrierRoutingMap** → default to All 3 (most permissive)
