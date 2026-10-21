package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBillingServiceTier(t *testing.T) {
	tests := []struct {
		name       string
		requested  string
		observed   string
		billing    string
		downgraded bool
	}{
		{name: "openai priority served as default", requested: "priority", observed: "default", billing: "default", downgraded: true},
		{name: "anthropic fast served as standard", requested: "fast", observed: "standard", billing: "standard", downgraded: true},
		{name: "ultrafast served as priority", requested: "ultrafast", observed: "priority", billing: "priority", downgraded: true},
		{name: "ultrafast served as default", requested: "ultrafast", observed: "default", billing: "default", downgraded: true},
		{name: "ultrafast honoured", requested: "ultrafast", observed: "ultrafast", billing: "ultrafast"},
		{name: "ultrafast missing declaration", requested: "ultrafast", billing: "ultrafast"},
		{name: "response cannot upgrade to ultrafast", requested: "priority", observed: "ultrafast", billing: "priority"},
		{name: "priority honoured", requested: "priority", observed: "priority", billing: "priority"},
		{name: "no declaration keeps request", requested: "priority", observed: "", billing: "priority"},
		{name: "no request no declaration", requested: "", observed: "", billing: ""},
		{name: "response never raises the tier", requested: "", observed: "priority", billing: ""},
		{name: "flex never raised to default", requested: "flex", observed: "default", billing: "flex"},
		{name: "default echoed for untiered request", requested: "", observed: "default", billing: ""},
		{name: "unknown response tier ignored", requested: "priority", observed: "turbo", billing: "priority"},
		{name: "case and whitespace normalised", requested: " Priority ", observed: "DEFAULT", billing: "default", downgraded: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBillingServiceTier(tt.requested, tt.observed)
			require.Equal(t, tt.billing, got.Billing)
			require.Equal(t, tt.downgraded, got.Downgraded)
		})
	}
}

func TestApplyServiceTierBillingResolutionOnlyRewritesDowngrades(t *testing.T) {
	t.Run("codex exception only covers OpenAI default", func(t *testing.T) {
		require.True(t, codexOAuthResponseTierIsNonAuthoritative("default"))
		require.False(t, codexOAuthResponseTierIsNonAuthoritative("standard"))
		require.False(t, codexOAuthResponseTierIsNonAuthoritative("flex"))
	})

	t.Run("public openai apikey downgrade rewrites tier", func(t *testing.T) {
		// No custom base_url: the account calls https://api.openai.com, whose
		// service_tier is the tier actually served.
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result)
		require.True(t, resolution.Downgraded)
		require.NotNil(t, result.ServiceTier)
		require.Equal(t, "default", *result.ServiceTier)
	})

	t.Run("explicit public openai base url still trusts default", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(openAIAPIKeyAccount("https://api.openai.com/v1"), result)
		require.True(t, resolution.Downgraded)
		require.Equal(t, "default", resolution.Billing)
		require.Equal(t, "default", *result.ServiceTier)
	})

	t.Run("openai honoured tier keeps pointer", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "priority"}
		require.False(t, ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result).Downgraded)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("openai untiered request stays nil", func(t *testing.T) {
		result := &OpenAIForwardResult{UpstreamResponseServiceTier: "priority"}
		require.False(t, ApplyOpenAIServiceTierBillingResolution(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, result).Downgraded)
		require.Nil(t, result.ServiceTier)
	})

	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run("codex "+accountType+" keeps outbound priority despite default echo", func(t *testing.T) {
			requested := "priority"
			result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
			resolution := ApplyOpenAIServiceTierBillingResolution(
				&Account{Platform: PlatformOpenAI, Type: accountType},
				result,
			)
			require.False(t, resolution.Downgraded)
			require.Equal(t, "priority", resolution.Requested)
			require.Equal(t, "default", resolution.Observed)
			require.Equal(t, "priority", resolution.Billing)
			require.Same(t, &requested, result.ServiceTier)
		})

		t.Run("codex "+accountType+" still accepts an explicit flex downgrade", func(t *testing.T) {
			requested := "priority"
			result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "flex"}
			resolution := ApplyOpenAIServiceTierBillingResolution(
				&Account{Platform: PlatformOpenAI, Type: accountType},
				result,
			)
			require.True(t, resolution.Downgraded)
			require.Equal(t, "flex", resolution.Billing)
			require.Equal(t, "flex", *result.ServiceTier)
		})

		t.Run("codex "+accountType+" response never promotes an untiered request", func(t *testing.T) {
			result := &OpenAIForwardResult{UpstreamResponseServiceTier: "priority"}
			resolution := ApplyOpenAIServiceTierBillingResolution(
				&Account{Platform: PlatformOpenAI, Type: accountType},
				result,
			)
			require.False(t, resolution.Downgraded)
			require.Empty(t, resolution.Billing)
			require.Nil(t, result.ServiceTier)
		})
	}

	t.Run("non-openai oauth still uses the generic response contract", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(
			&Account{Platform: PlatformGrok, Type: AccountTypeOAuth},
			result,
		)
		require.True(t, resolution.Downgraded)
		require.Equal(t, "default", resolution.Billing)
		require.Equal(t, "default", *result.ServiceTier)
	})

	t.Run("anthropic standard speed rewrites fast", func(t *testing.T) {
		requested := "fast"
		result := &ForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "standard"}
		require.True(t, ApplyForwardServiceTierBillingResolution(result).Downgraded)
		require.Equal(t, "standard", *result.ServiceTier)
	})

	t.Run("nil results are ignored", func(t *testing.T) {
		require.False(t, ApplyOpenAIServiceTierBillingResolution(nil, nil).Downgraded)
		require.False(t, ApplyForwardServiceTierBillingResolution(nil).Downgraded)
	})
}

func TestResolveOpenAIServiceTierBillingAPIKeyRelay(t *testing.T) {
	relay := openAIAPIKeyAccount("https://quanquanji.me/v1")

	t.Run("requested priority and observed default bills priority", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relay, result)
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Requested)
		require.Equal(t, "default", resolution.Observed)
		require.Equal(t, "priority", resolution.Billing)
		require.Same(t, &requested, result.ServiceTier)
	})

	t.Run("default echo is matched case-insensitively", func(t *testing.T) {
		resolution := ResolveOpenAIServiceTierBilling(relay, " priority ", " DEFAULT ")
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Billing)
		require.Equal(t, "default", resolution.Observed)
	})

	t.Run("explicit flex downgrade still lowers the bill", func(t *testing.T) {
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "flex"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relay, result)
		require.True(t, resolution.Downgraded)
		require.Equal(t, "flex", resolution.Billing)
		require.Equal(t, "flex", *result.ServiceTier)
	})

	t.Run("missing response tier keeps the request", func(t *testing.T) {
		resolution := ResolveOpenAIServiceTierBilling(relay, "priority", "")
		require.False(t, resolution.Downgraded)
		require.Equal(t, "priority", resolution.Billing)
	})

	t.Run("response never promotes an untiered request", func(t *testing.T) {
		result := &OpenAIForwardResult{UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(relay, result)
		require.False(t, resolution.Downgraded)
		require.Empty(t, resolution.Billing)
		require.Nil(t, result.ServiceTier)
	})

	t.Run("legacy upstream-type relay keeps priority", func(t *testing.T) {
		account := &Account{
			Platform:    PlatformOpenAI,
			Type:        AccountTypeUpstream,
			Credentials: map[string]any{"base_url": "https://relay.example.test/v1", "api_key": "sk-test"},
		}
		resolution := ResolveOpenAIServiceTierBilling(account, "fast", "default")
		require.False(t, resolution.Downgraded)
		require.Equal(t, "fast", resolution.Billing)
	})

	t.Run("non-openai custom base url still trusts default", func(t *testing.T) {
		account := &Account{
			Platform:    PlatformGrok,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"base_url": "https://quanquanji.me/v1"},
		}
		requested := "priority"
		result := &OpenAIForwardResult{ServiceTier: &requested, UpstreamResponseServiceTier: "default"}
		resolution := ApplyOpenAIServiceTierBillingResolution(account, result)
		require.True(t, resolution.Downgraded)
		require.Equal(t, "default", resolution.Billing)
	})
}

func TestOpenAIPublicAPIServiceTierHost(t *testing.T) {
	authoritative := []string{
		"",
		"https://api.openai.com",
		"https://api.openai.com/v1",
		"https://API.OpenAI.com/v1/",
		" https://api.openai.com/v1 ",
		"https://api.openai.com.",
		"https://user:pass@api.openai.com/v1",
		"https://region.api.openai.com/v1",
		"api.openai.com/v1",
	}
	for _, baseURL := range authoritative {
		t.Run("authoritative "+baseURL, func(t *testing.T) {
			require.True(t, openAIPublicAPIServiceTierIsAuthoritative(baseURL), baseURL)
			if baseURL == "" {
				return
			}
			resolution := ResolveOpenAIServiceTierBilling(openAIAPIKeyAccount(baseURL), "priority", "default")
			require.True(t, resolution.Downgraded)
			require.Equal(t, "default", resolution.Billing)
		})
	}

	relays := []string{
		"https://quanquanji.me/v1",
		"https://api.delicious-chicken.example/v1",
		"https://api.openai.com.evil.example/v1",
		"https://notapi.openai.com/v1",
		"https://my-resource.openai.azure.com/openai",
		"http://127.0.0.1:8080/v1",
		"://not a url",
	}
	for _, baseURL := range relays {
		t.Run("relay "+baseURL, func(t *testing.T) {
			require.False(t, openAIPublicAPIServiceTierIsAuthoritative(baseURL), baseURL)
			resolution := ResolveOpenAIServiceTierBilling(openAIAPIKeyAccount(baseURL), "priority", "default")
			require.False(t, resolution.Downgraded)
			require.Equal(t, "priority", resolution.Billing)
			require.Equal(t, "default", resolution.Observed)
		})
	}
}

func openAIAPIKeyAccount(baseURL string) *Account {
	return &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": baseURL,
		},
	}
}
