package nutrition

// MealTarget is the share of the daily target allotted to one meal slot.
type MealTarget struct {
	Slot     string  `json:"slot"`
	Calories float64 `json:"calories"`
	ProteinG float64 `json:"protein_g"`
	FatG     float64 `json:"fat_g"`
	CarbsG   float64 `json:"carbs_g"`
}

// SplitEven divides a daily target evenly across the given meal slots, in order.
// Returns nil when there are no slots.
func SplitEven(t Targets, slots []string) []MealTarget {
	n := len(slots)
	if n == 0 {
		return nil
	}
	f := 1.0 / float64(n)
	out := make([]MealTarget, n)
	for i, slot := range slots {
		out[i] = MealTarget{
			Slot:     slot,
			Calories: t.Calories * f,
			ProteinG: t.ProteinG * f,
			FatG:     t.FatG * f,
			CarbsG:   t.CarbsG * f,
		}
	}
	return out
}
