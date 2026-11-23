//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestToModelPlazaGroupDTO_VideoReferenceStaysOutOfCustomerPrice(t *testing.T) {
	customer := 9.99
	input := 3e-6
	groups := []service.PlazaGroup{{
		ID: 55, Name: "视频分组", Platform: "openai", SubscriptionType: "standard", RateMultiplier: 1,
		Models: []service.PlazaModel{{
			Name: "seedance-2.0-720p", Platform: "openai",
			Pricing: &service.ChannelModelPricing{
				BillingMode:     service.BillingModePerRequest,
				PerRequestPrice: &customer,
				InputPrice:      &input,
			},
			OfficialPricing: &service.PlazaOfficialPricing{InputPrice: &input},
		}, {
			Name: "grok-imagine-video", Platform: "openai",
			Pricing: &service.ChannelModelPricing{
				BillingMode:     service.BillingModeVideo,
				PerRequestPrice: &customer,
			},
		}},
	}}
	service.AttachPlazaVideoReferences(groups, service.DefaultPlazaVideoReferenceCatalog())
	dto := toModelPlazaGroupDTO(&groups[0], map[int64]float64{55: 0.2}, nil)
	raw, err := json.Marshal(dto)
	require.NoError(t, err)
	body := string(raw)
	require.Contains(t, body, `"amount_cny":"4.97"`)
	require.Contains(t, body, `"currency":"CNY"`)
	require.Contains(t, body, `"estimate":true`)
	require.Contains(t, body, `"official_bills_tokens":true`)
	require.Contains(t, body, "https://ark.volcengine.com/region:cn-beijing/docs/model-pricing")
	require.Contains(t, body, `"per_request_price":9.99`)
	require.NotContains(t, body, `"per_request_price":4.97`)
	require.NotContains(t, body, `"amount_cny":"0"`)
	require.InDelta(t, 1, dto.RateMultiplier, 0)
	require.InDelta(t, 0.2, *dto.UserRateMultiplier, 0)

	models := map[string]map[string]any{}
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	for _, item := range decoded["models"].([]any) {
		model := item.(map[string]any)
		models[model["name"].(string)] = model
	}
	_, grokHasRef := models["grok-imagine-video"]["video_reference"]
	require.False(t, grokHasRef, "Grok keeps its existing empty reference")
	require.Contains(t, models["seedance-2.0-720p"], "official_pricing")
	require.Equal(t, input, models["seedance-2.0-720p"]["official_pricing"].(map[string]any)["input_price"])
}

func TestOrderModelPlazaGroups_PersonalMultiplierThenInsertion(t *testing.T) {
	user := 0.05
	groups := []modelPlazaGroup{
		{ID: 55, Name: "视频分组", RateMultiplier: 1},
		{ID: 46, Name: "Grok分组", RateMultiplier: 0.2, UserRateMultiplier: &user},
		{ID: 53, Name: "GPT特惠分组0.2", RateMultiplier: 0.2},
		{ID: 7, Name: "低价", RateMultiplier: 0.1},
	}
	ordered := orderModelPlazaGroups(groups, []service.PlazaGroupInsertAfter{{GroupID: 55, AfterGroupID: 53}})
	require.Equal(t, []int64{46, 7, 53, 55}, plazaIDs(ordered))
	require.InDelta(t, 0.2, ordered[0].RateMultiplier, 0)
	require.InDelta(t, 0.05, *ordered[0].UserRateMultiplier, 0)
	require.Nil(t, ordered[3].UserRateMultiplier)
}

func TestModelPlazaHandler_DisplayOrdersGroupsAndKeepsCustomerPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &plazaDisplaySettingRepo{values: map[string]string{
		service.SettingKeyModelPlazaEnabled: "true",
		service.SettingKeyModelPlazaDisplay: `{"insert_after":[{"group_id":55,"after_group_id":53}]}`,
	}}
	h := NewModelPlazaHandler(
		service.NewModelPlazaService(plazaDisplayChannelRepo{}, plazaDisplayGroupRepo{}, nil, nil, nil, nil),
		nil,
		service.NewSettingService(repo, &config.Config{}),
	)
	r := gin.New()
	r.GET("/api/v1/model-plaza", h.Get)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var envelope struct {
		Data struct {
			GroupInsertAfter []service.PlazaGroupInsertAfter `json:"group_insert_after"`
			Groups           []struct {
				ID     int64   `json:"id"`
				Name   string  `json:"name"`
				Rate   float64 `json:"rate_multiplier"`
				Models []struct {
					Name    string          `json:"name"`
					Pricing json.RawMessage `json:"pricing"`
					Video   json.RawMessage `json:"video_reference"`
				} `json:"models"`
			} `json:"groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, []service.PlazaGroupInsertAfter{{GroupID: 55, AfterGroupID: 53}}, envelope.Data.GroupInsertAfter)
	got := make([]int64, 0, len(envelope.Data.Groups))
	for _, g := range envelope.Data.Groups {
		got = append(got, g.ID)
		require.Contains(t, []float64{0.1, 0.2, 1}, g.Rate)
	}
	require.Equal(t, []int64{7, 53, 55, 46}, got)
	var sawSeedance bool
	for _, g := range envelope.Data.Groups {
		if g.ID != 55 {
			continue
		}
		require.Len(t, g.Models, 1)
		require.Contains(t, string(g.Models[0].Pricing), `"per_request_price":8`)
		require.Contains(t, string(g.Models[0].Video), `"amount_cny":"4.97"`)
		require.NotContains(t, string(g.Models[0].Pricing), "4.97")
		sawSeedance = true
	}
	require.True(t, sawSeedance)
}

func TestModelPlazaAdminDisplay_RoundTripDoesNotWriteOtherSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &plazaDisplaySettingRepo{values: map[string]string{"group_rate_unrelated": "0.28"}}
	h := NewModelPlazaHandler(nil, nil, service.NewSettingService(repo, &config.Config{}))
	r := gin.New()
	r.GET("/admin/settings/model-plaza-display", h.AdminGetDisplay)
	r.PUT("/admin/settings/model-plaza-display", h.AdminUpdateDisplay)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/settings/model-plaza-display", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"video_reference_source":"builtin"`)
	require.Contains(t, w.Body.String(), `"amount_cny":"0.37"`)

	body := []byte(`{"insert_after":[{"group_id":55,"after_group_id":53}],"video_reference_override":{"currency":"USD","estimate":true,"official_bills_tokens":true,"source_url":"https://example.com","source_updated_on":"2026-11-19","checked_at":"2026-11-23","checked_timezone":"Asia/Shanghai","assumptions_zh":"估算","assumptions_en":"estimate","rows":[{"model":"seedance-2.0-720p","resolution":"720p","unit":"per_video","amount_cny":"0"}]}}`)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/settings/model-plaza-display", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Equal(t, "0.28", repo.values["group_rate_unrelated"])
	_, wrote := repo.values[service.SettingKeyModelPlazaDisplay]
	require.False(t, wrote)
}

type plazaDisplaySettingRepo struct {
	values map[string]string
}

func (r *plazaDisplaySettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (r *plazaDisplaySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	v, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return v, nil
}
func (r *plazaDisplaySettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}
func (r *plazaDisplaySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}
func (r *plazaDisplaySettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *plazaDisplaySettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *plazaDisplaySettingRepo) Delete(context.Context, string) error { return nil }

type plazaDisplayChannelRepo struct{ service.ChannelRepository }

func (plazaDisplayChannelRepo) ListAll(context.Context) ([]service.Channel, error) {
	price := 8.0
	return []service.Channel{{
		ID: 3, Name: "视频分组", Status: service.StatusActive, GroupIDs: []int64{53, 55, 46, 7},
		ModelPricing: []service.ChannelModelPricing{{
			Platform:        "openai",
			Models:          []string{"seedance-2.0-720p"},
			BillingMode:     service.BillingModePerRequest,
			PerRequestPrice: &price,
		}},
	}}, nil
}

type plazaDisplayGroupRepo struct{ service.GroupRepository }

func (plazaDisplayGroupRepo) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{
		{ID: 55, Name: "视频分组", Platform: "openai", RateMultiplier: 1, Status: service.StatusActive, SortOrder: 0},
		{ID: 46, Name: "Grok分组", Platform: "openai", RateMultiplier: 0.2, Status: service.StatusActive, SortOrder: 0},
		{ID: 53, Name: "GPT特惠分组0.2", Platform: "openai", RateMultiplier: 0.2, Status: service.StatusActive, SortOrder: 0},
		{ID: 7, Name: "低价", Platform: "openai", RateMultiplier: 0.1, Status: service.StatusActive, SortOrder: 9},
	}, nil
}

func plazaIDs(groups []modelPlazaGroup) []int64 {
	out := make([]int64, len(groups))
	for i := range groups {
		out[i] = groups[i].ID
	}
	return out
}
