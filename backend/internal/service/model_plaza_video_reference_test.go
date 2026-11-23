//go:build unit

package service

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultPlazaVideoReferenceCatalog_VerifiedSeedanceRows(t *testing.T) {
	catalog, err := NormalizePlazaVideoReferenceCatalog(DefaultPlazaVideoReferenceCatalog())
	require.NoError(t, err)
	require.Equal(t, PlazaVideoReferenceCurrencyCNY, catalog.Currency)
	require.True(t, catalog.Estimate)
	require.True(t, catalog.OfficialBillsTokens)
	require.Equal(t, "https://ark.volcengine.com/region:cn-beijing/docs/model-pricing", catalog.SourceURL)
	require.Equal(t, "2026-11-19", catalog.SourceUpdatedOn)
	require.Equal(t, "2026-11-23", catalog.CheckedAt)
	require.Equal(t, "Asia/Shanghai", catalog.CheckedTimezone)
	require.Contains(t, catalog.AssumptionsZH, "16:9")
	require.Contains(t, catalog.AssumptionsZH, "5 秒")
	require.Contains(t, catalog.AssumptionsZH, "无输入视频")
	require.Contains(t, catalog.AssumptionsZH, "不是固定收费标准")
	require.Contains(t, catalog.AssumptionsEN, "not a fixed tariff")
	require.NotContains(t, catalog.AssumptionsZH, "USD")

	want := []PlazaVideoReferenceRow{
		{Model: "seedance-2.0-720p", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerVideo, AmountCNY: "4.97"},
		{Model: "seedance-2.0-1080p", Resolution: "1080p", Unit: PlazaVideoReferenceUnitPerVideo, AmountCNY: "12.39"},
		{Model: "seedance-2.5-720p", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerVideo, AmountCNY: "7.56"},
		{Model: "seedance-2.5-1080p", Resolution: "1080p", Unit: PlazaVideoReferenceUnitPerVideo, AmountCNY: "18.71"},
		{Model: "seedance-2.0-fast", Resolution: "480p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.37"},
		{Model: "seedance-2.0-fast", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.80"},
		{Model: "seedance-2.0-mini", Resolution: "480p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.23"},
		{Model: "seedance-2.0-mini", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.50"},
		{Model: "seedance-2.0-per-second", Resolution: "480p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.46"},
		{Model: "seedance-2.0-per-second", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.99"},
		{Model: "seedance-2.0-per-second", Resolution: "1080p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "2.48"},
		{Model: "seedance-2.5-per-second", Resolution: "480p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "0.67"},
		{Model: "seedance-2.5-per-second", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "1.51"},
		{Model: "seedance-2.5-per-second", Resolution: "1080p", Unit: PlazaVideoReferenceUnitPerSecond, AmountCNY: "3.74"},
	}
	require.Equal(t, want, catalog.Rows)

	for _, absent := range []string{
		"grok-imagine-video", "grok", "minimax-video", "wan2.1", "vidu", "gemini-video", "muse",
		"seedance-2.0", "seedance-2.0-4k", "seedance-flash",
	} {
		require.Nil(t, referenceFor(catalog, absent), absent)
	}
}

func TestNormalizePlazaVideoReferenceCatalog_RejectsZeroAndForeignCurrency(t *testing.T) {
	base := DefaultPlazaVideoReferenceCatalog()
	base.Rows = []PlazaVideoReferenceRow{{Model: "seedance-2.0-720p", Resolution: "720p", Unit: PlazaVideoReferenceUnitPerVideo, AmountCNY: "0"}}
	_, err := NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)

	base.Rows[0].AmountCNY = "0.00"
	_, err = NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)

	base.Rows[0].AmountCNY = "-1.00"
	_, err = NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)

	base.Rows[0].AmountCNY = "4.975"
	_, err = NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)

	base = DefaultPlazaVideoReferenceCatalog()
	base.Currency = "USD"
	_, err = NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)

	base = DefaultPlazaVideoReferenceCatalog()
	base.Estimate = false
	_, err = NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)

	base = DefaultPlazaVideoReferenceCatalog()
	for i := range base.Rows {
		if base.Rows[i].Model == "seedance-2.0-fast" && base.Rows[i].Resolution == "720p" {
			base.Rows[i].Unit = PlazaVideoReferenceUnitPerVideo
		}
	}
	_, err = NormalizePlazaVideoReferenceCatalog(base)
	require.Error(t, err)
}

func TestAttachPlazaVideoReferences_DoesNotMutateCustomerPrices(t *testing.T) {
	customer := 9.99
	input := 1.25e-6
	pricing := &ChannelModelPricing{
		BillingMode:     BillingModePerRequest,
		PerRequestPrice: &customer,
		InputPrice:      &input,
		Models:          []string{"seedance-2.0-720p"},
	}
	before, err := json.Marshal(pricing)
	require.NoError(t, err)
	groupRate := 0.28
	videoRate := 1.5
	groups := []PlazaGroup{{
		ID:                  55,
		Name:                "视频分组",
		RateMultiplier:      groupRate,
		VideoRateMultiplier: videoRate,
		Models: []PlazaModel{{
			Name:            "Seedance-2.0-720p",
			Platform:        "openai",
			Pricing:         pricing,
			OfficialPricing: &PlazaOfficialPricing{InputPrice: &input},
		}, {
			Name:    "minimax-video-01",
			Pricing: &ChannelModelPricing{BillingMode: BillingModeVideo, PerRequestPrice: &customer},
		}},
	}}
	officialBefore := *groups[0].Models[0].OfficialPricing

	AttachPlazaVideoReferences(groups, DefaultPlazaVideoReferenceCatalog())

	after, err := json.Marshal(pricing)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	require.InDelta(t, customer, *groups[0].Models[0].Pricing.PerRequestPrice, 0)
	require.InDelta(t, input, *groups[0].Models[0].Pricing.InputPrice, 0)
	require.InDelta(t, groupRate, groups[0].RateMultiplier, 0)
	require.InDelta(t, videoRate, groups[0].VideoRateMultiplier, 0)
	require.Equal(t, officialBefore, *groups[0].Models[0].OfficialPricing)
	ref := groups[0].Models[0].VideoReference
	require.NotNil(t, ref)
	require.Equal(t, "CNY", ref.Currency)
	require.True(t, ref.Estimate)
	require.Equal(t, []PlazaVideoReferencePrice{{
		Resolution: "720p", Unit: PlazaVideoReferenceUnitPerVideo, AmountCNY: "4.97",
	}}, ref.Rows)
	require.Nil(t, groups[0].Models[1].VideoReference, "unsupported aliases stay blank")
}

func TestBillingSourcesDoNotReadPlazaVideoReferences(t *testing.T) {
	files := []string{
		"billing_service.go",
		"video_billing.go",
		"model_pricing_resolver.go",
		"gateway_request_pricing.go",
		"gateway_usage_billing.go",
	}
	needles := []string{"4.97", "18.71", "PlazaVideoReference", "model_plaza_display", "seedance-2.0-720p"}
	for _, name := range files {
		body, err := os.ReadFile(name)
		require.NoError(t, err)
		text := string(body)
		for _, needle := range needles {
			require.NotContainsf(t, text, needle, "%s must not participate in billing", name)
		}
	}
}

func referenceFor(catalog PlazaVideoReferenceCatalog, model string) *PlazaVideoReference {
	groups := []PlazaGroup{{Models: []PlazaModel{{Name: model}}}}
	AttachPlazaVideoReferences(groups, catalog)
	return groups[0].Models[0].VideoReference
}

func TestListPlazaGroups_IgnoresGroupSortOrder(t *testing.T) {
	channels := []Channel{plazaPricedChannel(1, "ch", []int64{10, 20}, "anthropic", "claude-sonnet")}
	groups := []Group{
		{ID: 10, Name: "later-rate", Platform: "anthropic", RateMultiplier: 1, SortOrder: 0},
		{ID: 20, Name: "earlier-rate", Platform: "anthropic", RateMultiplier: 0.2, SortOrder: 100},
	}
	out, err := newPlazaService(channels, groups, nil).ListGroups(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"earlier-rate", "later-rate"}, []string{out[0].Name, out[1].Name})
}

func TestBillingSourcesDoNotContainReferenceAmountsInComments(t *testing.T) {
	body, err := os.ReadFile("model_plaza_video_reference.go")
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "must not be"))
}
