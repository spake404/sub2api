package service

import (
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strings"
)

const (
	// PlazaVideoReferenceCurrencyCNY is the only currency stored for video
	// reference rows. Values are not converted from USD.
	PlazaVideoReferenceCurrencyCNY = "CNY"
	// PlazaVideoReferenceUnitPerVideo is CNY per generated video (条/次).
	PlazaVideoReferenceUnitPerVideo = "per_video"
	// PlazaVideoReferenceUnitPerSecond is CNY per second of generated video.
	PlazaVideoReferenceUnitPerSecond = "per_second"

	plazaVideoReferenceSourceURL       = "https://ark.volcengine.com/region:cn-beijing/docs/model-pricing"
	plazaVideoReferenceSourceUpdated   = "2026-11-19"
	plazaVideoReferenceCheckedAt       = "2026-11-23"
	plazaVideoReferenceCheckedTimezone = "Asia/Shanghai"
)

// PlazaVideoReferencePrice is one resolution row of a display-only estimate.
// AmountCNY is a canonical two-decimal string. Zero is never a stand-in for
// "unknown": missing rows are omitted entirely.
type PlazaVideoReferencePrice struct {
	Resolution string `json:"resolution"`
	Unit       string `json:"unit"`
	AmountCNY  string `json:"amount_cny"`
}

// PlazaVideoReference is attached to a plaza model for display. It must not be
// copied into ChannelModelPricing, group model pricing, or any billing quote.
type PlazaVideoReference struct {
	Currency            string                     `json:"currency"`
	Estimate            bool                       `json:"estimate"`
	OfficialBillsTokens bool                       `json:"official_bills_tokens"`
	SourceURL           string                     `json:"source_url"`
	SourceUpdatedOn     string                     `json:"source_updated_on"`
	CheckedAt           string                     `json:"checked_at"`
	CheckedTimezone     string                     `json:"checked_timezone"`
	AssumptionsZH       string                     `json:"assumptions_zh"`
	AssumptionsEN       string                     `json:"assumptions_en"`
	Rows                []PlazaVideoReferencePrice `json:"rows"`
}

// PlazaVideoReferenceCatalog is the admin-editable override document.
// An absent override selects the built-in verified seed.
type PlazaVideoReferenceCatalog struct {
	Currency            string                   `json:"currency"`
	Estimate            bool                     `json:"estimate"`
	OfficialBillsTokens bool                     `json:"official_bills_tokens"`
	SourceURL           string                   `json:"source_url"`
	SourceUpdatedOn     string                   `json:"source_updated_on"`
	CheckedAt           string                   `json:"checked_at"`
	CheckedTimezone     string                   `json:"checked_timezone"`
	AssumptionsZH       string                   `json:"assumptions_zh"`
	AssumptionsEN       string                   `json:"assumptions_en"`
	Rows                []PlazaVideoReferenceRow `json:"rows"`
}

// PlazaVideoReferenceRow is one configured reference. Model is matched to the
// plaza alias exactly, ignoring case and surrounding space.
type PlazaVideoReferenceRow struct {
	Model      string `json:"model"`
	Resolution string `json:"resolution"`
	Unit       string `json:"unit"`
	AmountCNY  string `json:"amount_cny"`
}

var (
	plazaDatePattern       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	plazaResolutionPattern = regexp.MustCompile(`^[1-9][0-9]{2,3}p$`)
)

// DefaultPlazaVideoReferenceCatalog is the verified Seedance estimate seed.
// Every amount is CNY for 16:9, 5 seconds of output, and no input video.
// The provider bills tokens; these are published equivalent examples, not a
// fixed charging tariff. Unsupported aliases are simply absent.
func DefaultPlazaVideoReferenceCatalog() PlazaVideoReferenceCatalog {
	return PlazaVideoReferenceCatalog{
		Currency:            PlazaVideoReferenceCurrencyCNY,
		Estimate:            true,
		OfficialBillsTokens: true,
		SourceURL:           plazaVideoReferenceSourceURL,
		SourceUpdatedOn:     plazaVideoReferenceSourceUpdated,
		CheckedAt:           plazaVideoReferenceCheckedAt,
		CheckedTimezone:     plazaVideoReferenceCheckedTimezone,
		AssumptionsZH:       "按 16:9、输出 5 秒、无输入视频估算。官方按 token 计费，这些数字是官方公布的等价示例，不是固定收费标准。货币为人民币，未做汇率换算。",
		AssumptionsEN:       "Estimated for a 16:9 frame, 5 seconds of output, and no input video. The provider bills tokens; these figures are published equivalent examples, not a fixed tariff. Amounts are CNY and were not converted from another currency.",
		Rows: []PlazaVideoReferenceRow{
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
		},
	}
}

// NormalizePlazaVideoReferenceCatalog validates a catalog and returns a copy
// safe to store. Zero, negative, and unparsable amounts are rejected rather
// than coerced to 0.
func NormalizePlazaVideoReferenceCatalog(in PlazaVideoReferenceCatalog) (PlazaVideoReferenceCatalog, error) {
	out := in
	out.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if out.Currency != PlazaVideoReferenceCurrencyCNY {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference currency must be CNY and must not be converted")
	}
	if !in.Estimate {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference rows must be marked as estimates, not a fixed tariff")
	}
	if !in.OfficialBillsTokens {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference rows must record that the official bill is token-based")
	}
	out.Estimate = true
	out.OfficialBillsTokens = true
	source := strings.TrimSpace(in.SourceURL)
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference source_url must be an https URL")
	}
	out.SourceURL = source
	if !plazaDatePattern.MatchString(strings.TrimSpace(in.SourceUpdatedOn)) || !plazaDatePattern.MatchString(strings.TrimSpace(in.CheckedAt)) {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference dates must be YYYY-MM-DD")
	}
	out.SourceUpdatedOn = strings.TrimSpace(in.SourceUpdatedOn)
	out.CheckedAt = strings.TrimSpace(in.CheckedAt)
	out.CheckedTimezone = strings.TrimSpace(in.CheckedTimezone)
	if out.CheckedTimezone == "" {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference checked_timezone is required")
	}
	out.AssumptionsZH = strings.TrimSpace(in.AssumptionsZH)
	out.AssumptionsEN = strings.TrimSpace(in.AssumptionsEN)
	if out.AssumptionsZH == "" || out.AssumptionsEN == "" {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference assumptions are required in Chinese and English")
	}
	if len(in.Rows) > 200 {
		return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference catalog exceeds 200 rows")
	}
	out.Rows = make([]PlazaVideoReferenceRow, 0, len(in.Rows))
	seen := make(map[string]struct{}, len(in.Rows))
	unitByModel := make(map[string]string, len(in.Rows))
	for _, row := range in.Rows {
		model := strings.ToLower(strings.TrimSpace(row.Model))
		if model == "" || strings.Contains(model, "*") || len(model) > 128 {
			return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference model alias is invalid")
		}
		resolution := strings.ToLower(strings.TrimSpace(row.Resolution))
		if !plazaResolutionPattern.MatchString(resolution) {
			return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference resolution %q is invalid", row.Resolution)
		}
		unit := strings.TrimSpace(row.Unit)
		if unit != PlazaVideoReferenceUnitPerVideo && unit != PlazaVideoReferenceUnitPerSecond {
			return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference unit must be per_video or per_second")
		}
		if prev, ok := unitByModel[model]; ok && prev != unit {
			return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference model %s mixes per_video and per_second", model)
		}
		unitByModel[model] = unit
		amount, err := canonicalPlazaCNY(row.AmountCNY)
		if err != nil {
			return PlazaVideoReferenceCatalog{}, fmt.Errorf("video reference amount for %s %s: %w", model, resolution, err)
		}
		key := model + "\x00" + resolution
		if _, ok := seen[key]; ok {
			return PlazaVideoReferenceCatalog{}, fmt.Errorf("duplicate video reference for %s %s", model, resolution)
		}
		seen[key] = struct{}{}
		out.Rows = append(out.Rows, PlazaVideoReferenceRow{
			Model:      model,
			Resolution: resolution,
			Unit:       unit,
			AmountCNY:  amount,
		})
	}
	return out, nil
}

// canonicalPlazaCNY accepts a positive decimal with at most two fractional
// digits and returns the two-decimal form. 0 and values that would round to
// 0.00 are rejected.
func canonicalPlazaCNY(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ContainsAny(s, "eE+") {
		return "", fmt.Errorf("must be a positive CNY amount")
	}
	rat, ok := new(big.Rat).SetString(s)
	if !ok || rat.Sign() <= 0 {
		return "", fmt.Errorf("must be a positive CNY amount")
	}
	scaled := new(big.Rat).Mul(rat, big.NewRat(100, 1))
	if !scaled.IsInt() {
		return "", fmt.Errorf("must have at most two decimal places")
	}
	canon := rat.FloatString(2)
	if canon == "0.00" {
		return "", fmt.Errorf("must not be zero")
	}
	return canon, nil
}

// AttachPlazaVideoReferences fills VideoReference from catalog rows.
// Customer Pricing, group rates, and official token fields are not modified.
// Models without a row stay nil so the UI can show a blank instead of 0.
func AttachPlazaVideoReferences(groups []PlazaGroup, catalog PlazaVideoReferenceCatalog) {
	if len(groups) == 0 || len(catalog.Rows) == 0 {
		return
	}
	byModel := make(map[string][]PlazaVideoReferenceRow, len(catalog.Rows))
	for _, row := range catalog.Rows {
		key := strings.ToLower(strings.TrimSpace(row.Model))
		if key == "" || strings.TrimSpace(row.AmountCNY) == "" || row.AmountCNY == "0" || row.AmountCNY == "0.00" {
			continue
		}
		byModel[key] = append(byModel[key], row)
	}
	for gi := range groups {
		for mi := range groups[gi].Models {
			rows := byModel[strings.ToLower(strings.TrimSpace(groups[gi].Models[mi].Name))]
			if len(rows) == 0 {
				continue
			}
			prices := make([]PlazaVideoReferencePrice, 0, len(rows))
			for _, row := range rows {
				prices = append(prices, PlazaVideoReferencePrice{
					Resolution: row.Resolution,
					Unit:       row.Unit,
					AmountCNY:  row.AmountCNY,
				})
			}
			groups[gi].Models[mi].VideoReference = &PlazaVideoReference{
				Currency:            catalog.Currency,
				Estimate:            catalog.Estimate,
				OfficialBillsTokens: catalog.OfficialBillsTokens,
				SourceURL:           catalog.SourceURL,
				SourceUpdatedOn:     catalog.SourceUpdatedOn,
				CheckedAt:           catalog.CheckedAt,
				CheckedTimezone:     catalog.CheckedTimezone,
				AssumptionsZH:       catalog.AssumptionsZH,
				AssumptionsEN:       catalog.AssumptionsEN,
				Rows:                prices,
			}
		}
	}
}
