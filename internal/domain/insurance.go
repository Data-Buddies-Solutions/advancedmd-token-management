package domain

// RoutingRule determines which providers a patient can see based on their insurance.
type RoutingRule string

const (
	RoutingNotAccepted RoutingRule = "not_accepted"
	RoutingBachOnly    RoutingRule = "bach_only"
	RoutingBachLicht   RoutingRule = "bach_licht"
	RoutingAll         RoutingRule = "all_three"
)

// InsuranceEntry maps an insurance name to its AMD carrier ID and routing rule.
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
	"aetna epo north broward":     {CarrierID: "car40887", Routing: RoutingBachOnly},
	"cigna local plus":            {CarrierID: "car301345", Routing: RoutingBachOnly},
	"eye america aao":             {CarrierID: "car308627", Routing: RoutingBachOnly},
	"florida blue steward tier 1": {CarrierID: "car40897", Routing: RoutingBachOnly},
	"humana gold":                 {CarrierID: "car308175", Routing: RoutingBachOnly},
	"humana medicaid":             {CarrierID: "car303033", Routing: RoutingBachOnly},
	"humana medicare":             {CarrierID: "car40906", Routing: RoutingBachOnly},
	"humana ppo":                  {CarrierID: "car303062", Routing: RoutingBachOnly},
	"humana premier hmo":          {CarrierID: "car303061", Routing: RoutingBachOnly},
	"meritain health":             {CarrierID: "car301578", Routing: RoutingBachOnly},
	"molina medicare":             {CarrierID: "car40912", Routing: RoutingBachOnly},

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
	"miami childrens health plan":        {CarrierID: "car40907", Routing: RoutingAll},
	"molina medicaid":                    {CarrierID: "car40912", Routing: RoutingAll},
	"multiplan phcs":                     {CarrierID: "car301648", Routing: RoutingAll},
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

// CarrierRoutingMap maps AMD carrier IDs to routing rules for existing patients.
// Used when we get the carrier ID from demographics.
// For the 5 ambiguous carriers, we default to RoutingAll (most permissive).
var CarrierRoutingMap = map[string]RoutingRule{
	// NOT ACCEPTED (unambiguous carriers only)
	"car281648": RoutingNotAccepted, // DOCTORS HEALTHCARE PLANS INC
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

// ColumnsForRouting returns the allowed column IDs for a routing rule.
func ColumnsForRouting(rule RoutingRule) map[string]bool {
	switch rule {
	case RoutingNotAccepted:
		return nil
	case RoutingBachOnly:
		return map[string]bool{"1513": true}
	case RoutingBachLicht:
		return map[string]bool{"1513": true, "1551": true}
	default:
		return map[string]bool{"1513": true, "1551": true, "1550": true}
	}
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

// LookupInsurance looks up an insurance name and returns its entry.
// Uses NormalizeForLookup for tolerance of punctuation, casing, and spacing.
func LookupInsurance(name string) (InsuranceEntry, bool) {
	entry, ok := InsuranceNameMap[NormalizeForLookup(name)]
	return entry, ok
}

// RoutingForCarrierID returns the routing rule for a carrier ID from demographics.
// Returns the rule and whether the carrier is ambiguous (shared across tiers).
// Unknown carrier IDs default to RoutingAll (most permissive).
func RoutingForCarrierID(carrierID string) (RoutingRule, bool) {
	ambiguous := AmbiguousCarriers[carrierID]

	if rule, ok := CarrierRoutingMap[carrierID]; ok {
		return rule, ambiguous
	}

	// Unknown or ambiguous carriers default to all three
	return RoutingAll, ambiguous
}

// ParseRoutingRule converts a string back to a typed RoutingRule.
// Used by the availability handler to parse the routing param from the request.
func ParseRoutingRule(s string) RoutingRule {
	switch RoutingRule(s) {
	case RoutingNotAccepted:
		return RoutingNotAccepted
	case RoutingBachOnly:
		return RoutingBachOnly
	case RoutingBachLicht:
		return RoutingBachLicht
	case RoutingAll:
		return RoutingAll
	default:
		return RoutingAll
	}
}
