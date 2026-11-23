package service

// PlazaGroupInsertAfter moves GroupID to the position immediately after
// AfterGroupID once groups have been ordered by effective rate and name.
// It does not change rate multipliers. Rules whose groups are not both
// visible are skipped, leaving the legacy order of the remaining groups.
type PlazaGroupInsertAfter struct {
	GroupID      int64 `json:"group_id"`
	AfterGroupID int64 `json:"after_group_id"`
}

// PlazaGroupOrderKey is the display identity used to order plaza groups.
// SortOrder from the groups table is intentionally not an input: a column of
// zeros must not reshuffle the legacy rate-then-name order.
type PlazaGroupOrderKey struct {
	ID       int64
	Name     string
	Rate     float64
	UserRate *float64
}

// EffectivePlazaRate is the user-specific multiplier when one is set,
// including an explicit 0, otherwise the group multiplier.
func EffectivePlazaRate(rate float64, userRate *float64) float64 {
	if userRate != nil {
		return *userRate
	}
	return rate
}

// OrderPlazaGroups returns groups in presentation order.
// With no insertion rules the order is effective rate ascending, then name,
// preserving the previous tie behavior via a stable sort. Rules then move
// listed groups to sit directly after their anchors without reordering anyone else.
func OrderPlazaGroups[T any](groups []T, key func(T) PlazaGroupOrderKey, rules []PlazaGroupInsertAfter) []T {
	if len(groups) < 2 {
		return append([]T(nil), groups...)
	}
	order := make([]int, len(groups))
	for i := range order {
		order[i] = i
	}
	// Stable so equal rate and equal name keep the incoming relative order.
	stableSortPlaza(order, func(i, j int) bool {
		a := key(groups[i])
		b := key(groups[j])
		ra := EffectivePlazaRate(a.Rate, a.UserRate)
		rb := EffectivePlazaRate(b.Rate, b.UserRate)
		if ra != rb {
			return ra < rb
		}
		return a.Name < b.Name
	})
	order = applyPlazaInsertAfter(order, func(index int) int64 {
		return key(groups[index]).ID
	}, rules)
	out := make([]T, len(order))
	for i, at := range order {
		out[i] = groups[at]
	}
	return out
}

func stableSortPlaza(order []int, less func(i, j int) bool) {
	// Local insertion sort keeps the comparator obvious and stable for the
	// small group lists the plaza renders.
	for i := 1; i < len(order); i++ {
		item := order[i]
		j := i
		for j > 0 && less(item, order[j-1]) {
			order[j] = order[j-1]
			j--
		}
		order[j] = item
	}
}

func applyPlazaInsertAfter(order []int, idOf func(index int) int64, rules []PlazaGroupInsertAfter) []int {
	for _, rule := range rules {
		if rule.GroupID <= 0 || rule.AfterGroupID <= 0 || rule.GroupID == rule.AfterGroupID {
			continue
		}
		from := -1
		anchor := -1
		for i, at := range order {
			id := idOf(at)
			if id == rule.GroupID {
				from = i
			}
			if id == rule.AfterGroupID {
				anchor = i
			}
		}
		if from < 0 || anchor < 0 || from == anchor {
			continue
		}
		item := order[from]
		order = append(order[:from], order[from+1:]...)
		anchor = -1
		for i, at := range order {
			if idOf(at) == rule.AfterGroupID {
				anchor = i
				break
			}
		}
		if anchor < 0 {
			order = append(order, item)
			continue
		}
		insertAt := anchor + 1
		order = append(order, 0)
		copy(order[insertAt+1:], order[insertAt:])
		order[insertAt] = item
	}
	return order
}
