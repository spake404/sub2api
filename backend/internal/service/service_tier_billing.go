package service

import (
	"log/slog"
	"net/url"
	"strings"
)

// ServiceTierBillingResolution describes how the billable service tier of one
// request was settled between the tier the client asked for and the tier the
// upstream reports having used.
type ServiceTierBillingResolution struct {
	Requested  string // tier carried by the request sent upstream ("" when none)
	Observed   string // tier declared by the upstream response ("" when none)
	Billing    string // tier used for billing and the usage log
	Downgraded bool   // Billing is cheaper than Requested
}

// ResolveBillingServiceTier picks the tier to bill. The upstream declaration is
// trusted only to lower the bill: a request served below the tier it asked for
// (OpenAI "priority" answered with service_tier "default", Anthropic speed=fast
// answered with usage.speed "standard") is billed at the cheaper tier. A response
// that claims a more expensive tier, an unknown one, or none at all leaves the
// requested tier in place, which is also the behaviour for upstreams that never
// declare a tier.
func ResolveBillingServiceTier(requested, observed string) ServiceTierBillingResolution {
	requested = normalizeBillingServiceTier(requested)
	observed = normalizeBillingServiceTier(observed)
	resolution := ServiceTierBillingResolution{Requested: requested, Observed: observed, Billing: requested}
	if observed == "" || observed == requested {
		return resolution
	}
	observedRank, known := serviceTierCostRank(observed)
	if !known {
		return resolution
	}
	requestedRank, _ := serviceTierCostRank(requested)
	if observedRank >= requestedRank {
		return resolution
	}
	resolution.Billing = observed
	resolution.Downgraded = true
	return resolution
}

// serviceTierCostRank orders tiers by their cost relative to the base rate, so a
// lower rank is always cheaper. Unknown tiers rank as the base rate and report
// known=false so callers can refuse to act on them.
func serviceTierCostRank(tier string) (rank int, known bool) {
	switch normalizeBillingServiceTier(tier) {
	case "flex":
		return 0, true
	case "", "default", "standard", "auto", "scale":
		return 1, true
	case "priority", "fast":
		return 2, true
	case "ultrafast":
		// More expensive than priority in the upstream contract. The priced
		// multiplier is still the Fast 2x until a model defines otherwise.
		return 3, true
	default:
		return 1, false
	}
}

// ResolveOpenAIServiceTierBilling applies the response-tier contract for the
// selected credential.
//
// The public OpenAI API (api.openai.com, including an API-key account whose
// base_url is empty or points at that host) declares the tier it actually
// served. A cheaper observed tier, including default, lowers the bill.
//
// Two credentials are known to answer service_tier=default even when the
// outbound request was a higher tier and the upstream still charged Fast:
//
//   - ChatGPT Codex / OAuth-like accounts. The private endpoint commonly
//     echoes default for effective Fast turns.
//   - OpenAI API-key and legacy upstream-type relays whose base_url is not
//     the public API. Another Sub2API (quanquanji.me / delicious-chicken
//     style) bills the operator at Fast ×2 and still returns
//     "service_tier":"default" in the HTTP body. Trusting that echo bills
//     the customer at standard while the operator pays priority upstream.
//
// Only an observed default is non-authoritative on those credentials. An
// explicit cheaper tier such as flex still lowers the bill, and a response
// never raises the tier above what was requested.
func ResolveOpenAIServiceTierBilling(account *Account, requested, observed string) ServiceTierBillingResolution {
	if openAIObservedDefaultIsNonAuthoritative(account, observed) {
		return ServiceTierBillingResolution{
			Requested: normalizeBillingServiceTier(requested),
			Observed:  normalizeBillingServiceTier(observed),
			Billing:   normalizeBillingServiceTier(requested),
		}
	}
	return ResolveBillingServiceTier(requested, observed)
}

// codexOAuthResponseTierIsNonAuthoritative is the observed-value half of the
// exception. Only default is the known unreliable echo. standard, flex, and
// any other cheaper tier stay authoritative so a real downgrade still lowers
// the bill. The name is historical: API-key relays share this observed value.
func codexOAuthResponseTierIsNonAuthoritative(observed string) bool {
	switch normalizeBillingServiceTier(observed) {
	case "default":
		return true
	default:
		return false
	}
}

func openAIObservedDefaultIsNonAuthoritative(account *Account, observed string) bool {
	if account == nil || !codexOAuthResponseTierIsNonAuthoritative(observed) {
		return false
	}
	if account.IsOpenAIOAuthLike() {
		return true
	}
	return openAIAPIKeyRelayResponseDefaultIsNonAuthoritative(account)
}

// openAIAPIKeyRelayResponseDefaultIsNonAuthoritative reports OpenAI accounts
// that authenticate with a static key and call something other than the
// public API. Those relays cannot be distinguished from api.openai.com by
// the response body: the body says default either way, but only the public
// API's default is the tier that was actually served and billed.
func openAIAPIKeyRelayResponseDefaultIsNonAuthoritative(account *Account) bool {
	if account == nil || !account.IsOpenAI() {
		return false
	}
	switch account.Type {
	case AccountTypeAPIKey, AccountTypeUpstream:
	default:
		return false
	}
	return !openAIPublicAPIServiceTierIsAuthoritative(account.GetOpenAIBaseURL())
}

// openAIPublicAPIServiceTierIsAuthoritative reports whether baseURL is the
// public OpenAI API, whose service_tier may lower billing. An empty URL is
// authoritative because OpenAI accounts default to https://api.openai.com.
// Any other host — including Azure and third-party relays — is not: a relay
// echo of default is known to be a lie, and the body alone cannot prove a
// genuine capacity downgrade.
func openAIPublicAPIServiceTierIsAuthoritative(baseURL string) bool {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return true
	}
	host := openAIServiceTierBaseURLHost(raw)
	if host == "" {
		return false
	}
	return host == "api.openai.com" || strings.HasSuffix(host, ".api.openai.com")
}

func openAIServiceTierBaseURLHost(baseURL string) string {
	candidate := strings.TrimSpace(baseURL)
	if candidate == "" {
		return ""
	}
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
}

// ApplyOpenAIServiceTierBillingResolution lowers result.ServiceTier only when
// the selected credential's upstream response tier is authoritative.
func ApplyOpenAIServiceTierBillingResolution(account *Account, result *OpenAIForwardResult) ServiceTierBillingResolution {
	if result == nil {
		return ServiceTierBillingResolution{}
	}
	resolution := ResolveOpenAIServiceTierBilling(account, optionalStringValue(result.ServiceTier), result.UpstreamResponseServiceTier)
	if resolution.Downgraded {
		billing := resolution.Billing
		result.ServiceTier = &billing
	}
	return resolution
}

// ApplyForwardServiceTierBillingResolution is the ForwardResult counterpart of
// ApplyOpenAIServiceTierBillingResolution.
func ApplyForwardServiceTierBillingResolution(result *ForwardResult) ServiceTierBillingResolution {
	if result == nil {
		return ServiceTierBillingResolution{}
	}
	resolution := ResolveBillingServiceTier(optionalStringValue(result.ServiceTier), result.UpstreamResponseServiceTier)
	if resolution.Downgraded {
		billing := resolution.Billing
		result.ServiceTier = &billing
	}
	return resolution
}

// logServiceTierBillingDowngrade leaves an audit trail for every request billed
// below the tier it asked for; unchanged tiers are not logged.
func logServiceTierBillingDowngrade(component string, account *Account, requestID string, resolution ServiceTierBillingResolution) {
	if !resolution.Downgraded {
		return
	}
	attrs := []any{
		"component", component,
		"request_id", strings.TrimSpace(requestID),
		"requested_tier", resolution.Requested,
		"response_tier", resolution.Observed,
		"billed_tier", resolution.Billing,
	}
	if account != nil {
		attrs = append(attrs, "platform", account.Platform, "account_id", account.ID)
	}
	slog.Info("billing.service_tier_downgraded", attrs...)
}
