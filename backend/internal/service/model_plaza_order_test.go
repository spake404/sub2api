//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrderPlazaGroups_DefaultRateThenNameAndPersonalRates(t *testing.T) {
	userHalf := 0.5
	userZero := 0.0
	groups := []PlazaGroup{
		{ID: 55, Name: "视频分组", RateMultiplier: 1},
		{ID: 46, Name: "Grok分组", RateMultiplier: 0.2},
		{ID: 53, Name: "GPT特惠分组0.2", RateMultiplier: 0.2},
		{ID: 7, Name: "低价", RateMultiplier: 0.1},
		{ID: 8, Name: "同价乙", RateMultiplier: 0.2},
		{ID: 9, Name: "同价甲", RateMultiplier: 3, VideoRateMultiplier: 9},
	}
	// 个人倍率覆盖分组倍率，显式 0 也生效。sort_order 不在排序键里。
	rates := map[int64]*float64{9: &userHalf, 7: &userZero}
	ordered := OrderPlazaGroups(groups, func(g PlazaGroup) PlazaGroupOrderKey {
		return PlazaGroupOrderKey{ID: g.ID, Name: g.Name, Rate: g.RateMultiplier, UserRate: rates[g.ID]}
	}, nil)

	require.Equal(t, []int64{7, 53, 46, 8, 9, 55}, idsOf(ordered))
	require.InDelta(t, 1, orderedGroup(ordered, 55).RateMultiplier, 0)
	require.InDelta(t, 0.2, orderedGroup(ordered, 46).RateMultiplier, 0)
	require.InDelta(t, 0.2, orderedGroup(ordered, 53).RateMultiplier, 0)
	require.InDelta(t, 9, orderedGroup(ordered, 9).VideoRateMultiplier, 0)

	// 完全相同的倍率与名称保持输入相对顺序。
	tied := []PlazaGroup{
		{ID: 2, Name: "same", RateMultiplier: 1},
		{ID: 1, Name: "same", RateMultiplier: 1},
	}
	require.Equal(t, []int64{2, 1}, idsOf(OrderPlazaGroups(tied, func(g PlazaGroup) PlazaGroupOrderKey {
		return PlazaGroupOrderKey{ID: g.ID, Name: g.Name, Rate: g.RateMultiplier}
	}, nil)))
}

func TestOrderPlazaGroups_InsertsVideoGroupAfterGPTWithoutReorderingOthers(t *testing.T) {
	groups := []PlazaGroup{
		{ID: 90, Name: "高价", RateMultiplier: 2},
		{ID: 55, Name: "视频分组", RateMultiplier: 1},
		{ID: 46, Name: "Grok分组", RateMultiplier: 0.2},
		{ID: 53, Name: "GPT特惠分组0.2", RateMultiplier: 0.2},
		{ID: 7, Name: "低价", RateMultiplier: 0.1},
	}
	rules := []PlazaGroupInsertAfter{{GroupID: 55, AfterGroupID: 53}}
	ordered := OrderPlazaGroups(groups, plazaKey, rules)
	require.Equal(t, []int64{7, 53, 55, 46, 90}, idsOf(ordered))
	for _, g := range ordered {
		require.Equal(t, originalRate(groups, g.ID), g.RateMultiplier)
	}

	// 个人倍率会先改变其余分组的相对顺序，再把 55 插到 53 后面。
	user := 0.05
	rates := map[int64]*float64{46: &user}
	reordered := OrderPlazaGroups(groups, func(g PlazaGroup) PlazaGroupOrderKey {
		key := plazaKey(g)
		key.UserRate = rates[g.ID]
		return key
	}, rules)
	require.Equal(t, []int64{46, 7, 53, 55, 90}, idsOf(reordered))

	// 锚点不在可见列表时，被移动的分组留在原倍率位置。
	visible := []PlazaGroup{groups[1], groups[2], groups[4]}
	require.Equal(t, []int64{7, 46, 55}, idsOf(OrderPlazaGroups(visible, plazaKey, rules)))
}

func plazaKey(g PlazaGroup) PlazaGroupOrderKey {
	return PlazaGroupOrderKey{ID: g.ID, Name: g.Name, Rate: g.RateMultiplier}
}

func idsOf(groups []PlazaGroup) []int64 {
	out := make([]int64, len(groups))
	for i := range groups {
		out[i] = groups[i].ID
	}
	return out
}

func orderedGroup(groups []PlazaGroup, id int64) PlazaGroup {
	for _, g := range groups {
		if g.ID == id {
			return g
		}
	}
	return PlazaGroup{}
}

func originalRate(groups []PlazaGroup, id int64) float64 {
	return orderedGroup(groups, id).RateMultiplier
}
