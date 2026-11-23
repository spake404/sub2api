//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type memSettingRepo struct {
	values map[string]string
	writes []string
}

func (m *memSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (m *memSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if m.values == nil {
		return "", ErrSettingNotFound
	}
	v, ok := m.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (m *memSettingRepo) Set(_ context.Context, key, value string) error {
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value
	m.writes = append(m.writes, key)
	return nil
}
func (m *memSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if v, ok := m.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}
func (m *memSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (m *memSettingRepo) GetAll(context.Context) (map[string]string, error)    { return m.values, nil }
func (m *memSettingRepo) Delete(context.Context, string) error                 { return nil }

func TestUpdateModelPlazaDisplay_WritesOnlyDisplayKeyAndRejectsZero(t *testing.T) {
	repo := &memSettingRepo{values: map[string]string{
		SettingKeyModelPlazaEnabled: "true",
		"group_rate_unrelated":      "0.28",
	}}
	svc := NewSettingService(repo, &config.Config{})
	override := DefaultPlazaVideoReferenceCatalog()
	override.Rows[0].AmountCNY = "0"
	_, err := svc.UpdateModelPlazaDisplay(context.Background(), ModelPlazaDisplayUpdate{
		VideoReferenceOverride: &override,
		InsertAfter:            []PlazaGroupInsertAfter{{GroupID: 55, AfterGroupID: 53}},
	})
	require.Error(t, err)
	require.Empty(t, repo.writes)
	require.Equal(t, "0.28", repo.values["group_rate_unrelated"])

	override = DefaultPlazaVideoReferenceCatalog()
	view, err := svc.UpdateModelPlazaDisplay(context.Background(), ModelPlazaDisplayUpdate{
		VideoReferenceOverride: &override,
		InsertAfter:            []PlazaGroupInsertAfter{{GroupID: 55, AfterGroupID: 53}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{SettingKeyModelPlazaDisplay}, repo.writes)
	require.Equal(t, "override", view.VideoReferenceSource)
	require.Equal(t, "4.97", view.VideoReferences.Rows[0].AmountCNY)
	require.Equal(t, []PlazaGroupInsertAfter{{GroupID: 55, AfterGroupID: 53}}, view.InsertAfter)
	require.NotContains(t, repo.values[SettingKeyModelPlazaDisplay], "rate_multiplier")

	reset, err := svc.UpdateModelPlazaDisplay(context.Background(), ModelPlazaDisplayUpdate{
		ResetVideoReferences: true,
		InsertAfter:          nil,
	})
	require.NoError(t, err)
	require.Equal(t, "builtin", reset.VideoReferenceSource)
	require.Empty(t, reset.InsertAfter)
	require.Len(t, reset.VideoReferences.Rows, 14)
	stored := repo.values[SettingKeyModelPlazaDisplay]
	require.NotContains(t, stored, "video_reference_override")
}

func TestGetModelPlazaRuntime_InvalidDisplayFallsBackWithoutDisablingPlaza(t *testing.T) {
	repo := &memSettingRepo{values: map[string]string{
		SettingKeyModelPlazaEnabled: "true",
		SettingKeyModelPlazaDisplay: "{",
	}}
	rt := NewSettingService(repo, &config.Config{}).GetModelPlazaRuntime(context.Background())
	require.True(t, rt.Enabled)
	require.True(t, rt.DisplayInvalid)
	require.Nil(t, rt.Display.VideoReferenceOverride)
	require.Empty(t, rt.Display.InsertAfter)
	require.Len(t, rt.Display.EffectiveVideoCatalog().Rows, 14)
}

func TestParseModelPlazaDisplay_EmptyIsLegacyOrderAndBuiltinSeed(t *testing.T) {
	cfg, err := ParseModelPlazaDisplay("  ")
	require.NoError(t, err)
	require.Nil(t, cfg.VideoReferenceOverride)
	require.Empty(t, cfg.InsertAfter)
	raw, err := json.Marshal(cfg.View())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"video_reference_source":"builtin"`)
	require.Contains(t, string(raw), `"amount_cny":"4.97"`)
}
