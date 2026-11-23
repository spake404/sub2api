package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrModelPlazaDisplayInvalid means the stored display JSON cannot be applied.
// Callers must fall back to the built-in reference seed and the legacy group
// order. They must not treat the failure as a zero price.
var ErrModelPlazaDisplayInvalid = errors.New("model plaza display config is invalid")

// ModelPlazaDisplayConfig is the persisted plaza-only presentation document.
// VideoReferenceOverride nil selects the built-in seed. InsertAfter empty
// keeps the legacy rate-then-name order.
type ModelPlazaDisplayConfig struct {
	VideoReferenceOverride *PlazaVideoReferenceCatalog `json:"video_reference_override,omitempty"`
	InsertAfter            []PlazaGroupInsertAfter     `json:"insert_after,omitempty"`
}

// ModelPlazaDisplayUpdate is the admin write body. ResetVideoReferences drops
// an override and restores the built-in seed. InsertAfter replaces the rule
// list; an empty list restores the legacy order.
type ModelPlazaDisplayUpdate struct {
	ResetVideoReferences   bool                        `json:"reset_video_references"`
	VideoReferenceOverride *PlazaVideoReferenceCatalog `json:"video_reference_override"`
	InsertAfter            []PlazaGroupInsertAfter     `json:"insert_after"`
}

// ModelPlazaDisplayView is the admin read model. VideoReferences is always the
// catalog that the plaza will display.
type ModelPlazaDisplayView struct {
	VideoReferenceSource string                     `json:"video_reference_source"`
	InvalidReason        string                     `json:"invalid_reason,omitempty"`
	VideoReferences      PlazaVideoReferenceCatalog `json:"video_references"`
	InsertAfter          []PlazaGroupInsertAfter    `json:"insert_after"`
}

// EffectiveVideoCatalog returns the override or the built-in seed.
func (c ModelPlazaDisplayConfig) EffectiveVideoCatalog() PlazaVideoReferenceCatalog {
	if c.VideoReferenceOverride == nil {
		return DefaultPlazaVideoReferenceCatalog()
	}
	return *c.VideoReferenceOverride
}

// View describes the catalog and rules the plaza will use.
func (c ModelPlazaDisplayConfig) View() ModelPlazaDisplayView {
	source := "builtin"
	catalog := DefaultPlazaVideoReferenceCatalog()
	if c.VideoReferenceOverride != nil {
		source = "override"
		catalog = *c.VideoReferenceOverride
	}
	rules := c.InsertAfter
	if rules == nil {
		rules = []PlazaGroupInsertAfter{}
	}
	return ModelPlazaDisplayView{
		VideoReferenceSource: source,
		VideoReferences:      catalog,
		InsertAfter:          rules,
	}
}

// ParseModelPlazaDisplay parses a stored settings value. An empty value is the
// default config, not an error and not a zero price.
func ParseModelPlazaDisplay(raw string) (ModelPlazaDisplayConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return ModelPlazaDisplayConfig{}, nil
	}
	var cfg ModelPlazaDisplayConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ModelPlazaDisplayConfig{}, fmt.Errorf("%w: %s", ErrModelPlazaDisplayInvalid, err.Error())
	}
	if cfg.VideoReferenceOverride != nil {
		normalized, err := NormalizePlazaVideoReferenceCatalog(*cfg.VideoReferenceOverride)
		if err != nil {
			return ModelPlazaDisplayConfig{}, fmt.Errorf("%w: %s", ErrModelPlazaDisplayInvalid, err.Error())
		}
		cfg.VideoReferenceOverride = &normalized
	}
	rules, err := NormalizePlazaInsertAfter(cfg.InsertAfter)
	if err != nil {
		return ModelPlazaDisplayConfig{}, fmt.Errorf("%w: %s", ErrModelPlazaDisplayInvalid, err.Error())
	}
	cfg.InsertAfter = rules
	return cfg, nil
}

// NormalizePlazaInsertAfter checks presentation rules. It does not look up
// groups and does not change their rates. Unknown ids are kept so a rule can
// be saved before the group is visible; the sorter skips missing anchors.
func NormalizePlazaInsertAfter(rules []PlazaGroupInsertAfter) ([]PlazaGroupInsertAfter, error) {
	if len(rules) > 50 {
		return nil, fmt.Errorf("at most 50 group presentation rules are allowed")
	}
	out := make([]PlazaGroupInsertAfter, 0, len(rules))
	seen := make(map[int64]struct{}, len(rules))
	for _, rule := range rules {
		if rule.GroupID <= 0 || rule.AfterGroupID <= 0 {
			return nil, fmt.Errorf("group presentation ids must be positive")
		}
		if rule.GroupID == rule.AfterGroupID {
			return nil, fmt.Errorf("a group cannot be inserted after itself")
		}
		if _, ok := seen[rule.GroupID]; ok {
			return nil, fmt.Errorf("group %d has more than one presentation rule", rule.GroupID)
		}
		seen[rule.GroupID] = struct{}{}
		out = append(out, PlazaGroupInsertAfter{GroupID: rule.GroupID, AfterGroupID: rule.AfterGroupID})
	}
	return out, nil
}

// GetModelPlazaDisplay reads the plaza display setting. A missing row is the
// default config. Invalid JSON returns ErrModelPlazaDisplayInvalid.
func (s *SettingService) GetModelPlazaDisplay(ctx context.Context) (ModelPlazaDisplayConfig, error) {
	if s == nil || s.settingRepo == nil {
		return ModelPlazaDisplayConfig{}, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyModelPlazaDisplay)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return ModelPlazaDisplayConfig{}, nil
		}
		return ModelPlazaDisplayConfig{}, err
	}
	return ParseModelPlazaDisplay(raw)
}

// UpdateModelPlazaDisplay validates and stores plaza display config.
// The write touches only the display setting key.
func (s *SettingService) UpdateModelPlazaDisplay(ctx context.Context, update ModelPlazaDisplayUpdate) (ModelPlazaDisplayView, error) {
	if s == nil || s.settingRepo == nil {
		return ModelPlazaDisplayView{}, fmt.Errorf("setting repository is unavailable")
	}
	cfg := ModelPlazaDisplayConfig{}
	if !update.ResetVideoReferences && update.VideoReferenceOverride != nil {
		normalized, err := NormalizePlazaVideoReferenceCatalog(*update.VideoReferenceOverride)
		if err != nil {
			return ModelPlazaDisplayView{}, infraerrors.BadRequest("INVALID_MODEL_PLAZA_DISPLAY", err.Error())
		}
		cfg.VideoReferenceOverride = &normalized
	}
	rules, err := NormalizePlazaInsertAfter(update.InsertAfter)
	if err != nil {
		return ModelPlazaDisplayView{}, infraerrors.BadRequest("INVALID_MODEL_PLAZA_DISPLAY", err.Error())
	}
	cfg.InsertAfter = rules
	payload, err := json.Marshal(cfg)
	if err != nil {
		return ModelPlazaDisplayView{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyModelPlazaDisplay, string(payload)); err != nil {
		return ModelPlazaDisplayView{}, err
	}
	return cfg.View(), nil
}
