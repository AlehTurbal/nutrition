package nutrition

import "testing"

func TestSplitEven(t *testing.T) {
	tg := Targets{Calories: 2000, ProteinG: 150, FatG: 60, CarbsG: 200}

	slots := []string{"breakfast", "lunch", "dinner"}
	parts := SplitEven(tg, slots)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	for _, p := range parts {
		approx(t, "calories "+p.Slot, p.Calories, 2000.0/3)
		approx(t, "protein "+p.Slot, p.ProteinG, 50)
	}
	if parts[0].Slot != "breakfast" || parts[2].Slot != "dinner" {
		t.Errorf("slots not preserved in order: %v", parts)
	}

	// Sum of parts must equal the daily total.
	var sumCal float64
	for _, p := range parts {
		sumCal += p.Calories
	}
	approx(t, "sum calories", sumCal, 2000)
}

func TestSplitEvenEmpty(t *testing.T) {
	if got := SplitEven(Targets{Calories: 100}, nil); got != nil {
		t.Errorf("expected nil for no slots, got %v", got)
	}
}
