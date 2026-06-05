package mealplans

import (
	"math"
	"testing"
	"time"
)

func ptr(f float64) *float64 { return &f }

func mkDate(s string) Date {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return Date{Time: t}
}

func TestBuildShoppingListAggregatesAndScales(t *testing.T) {
	// Two breakfast occurrences of 100g chicken + one dinner of 50g rice.
	// Chicken is split across two days, rice falls on the second day.
	chickenKcal := ptr(165.0)
	d1, d2 := mkDate("2026-06-02"), mkDate("2026-06-01")
	lines := []line{
		{slot: "breakfast", date: d1, productID: 1, name: "Chicken", grams: 100, kcal100: chickenKcal, protein100: ptr(31), fat100: ptr(3.6), carbs100: ptr(0)},
		{slot: "breakfast", date: d2, productID: 1, name: "Chicken", grams: 100, kcal100: chickenKcal, protein100: ptr(31), fat100: ptr(3.6), carbs100: ptr(0)},
		{slot: "dinner", date: d2, productID: 2, name: "Rice", grams: 50, kcal100: ptr(130), protein100: ptr(2.7), fat100: ptr(0.3), carbs100: ptr(28)},
	}

	sl := buildShoppingList(lines)

	if len(sl.Items) != 2 {
		t.Fatalf("expected 2 products, got %d", len(sl.Items))
	}
	// Sorted by name: Chicken, Rice.
	if sl.Items[0].ProductName != "Chicken" || !eq(sl.Items[0].Grams, 200) {
		t.Errorf("chicken aggregate wrong: %+v", sl.Items[0])
	}
	if !eq(sl.Items[0].Macros.Kcal, 330) { // 165 * 2
		t.Errorf("chicken kcal = %.2f, want 330", sl.Items[0].Macros.Kcal)
	}

	// Totals = 330 (chicken) + 65 (rice 130*0.5).
	if !eq(sl.Totals.Kcal, 395) {
		t.Errorf("total kcal = %.2f, want 395", sl.Totals.Kcal)
	}

	if len(sl.BySlot) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(sl.BySlot))
	}
	// Alphabetical: breakfast, dinner.
	if sl.BySlot[0].Slot != "breakfast" || !eq(sl.BySlot[0].Macros.Kcal, 330) {
		t.Errorf("breakfast slot wrong: %+v", sl.BySlot[0])
	}
	if sl.BySlot[1].Slot != "dinner" || !eq(sl.BySlot[1].Macros.Kcal, 65) {
		t.Errorf("dinner slot wrong: %+v", sl.BySlot[1])
	}

	if len(sl.ByDay) != 2 {
		t.Fatalf("expected 2 days, got %d", len(sl.ByDay))
	}
	// Sorted chronologically: 2026-06-01 (chicken 165 + rice 65), 2026-06-02 (chicken 165).
	if sl.ByDay[0].Date.Format("2006-01-02") != "2026-06-01" || !eq(sl.ByDay[0].Macros.Kcal, 230) {
		t.Errorf("day[0] wrong: %s %+v", sl.ByDay[0].Date.Format("2006-01-02"), sl.ByDay[0].Macros)
	}
	if sl.ByDay[1].Date.Format("2006-01-02") != "2026-06-02" || !eq(sl.ByDay[1].Macros.Kcal, 165) {
		t.Errorf("day[1] wrong: %s %+v", sl.ByDay[1].Date.Format("2006-01-02"), sl.ByDay[1].Macros)
	}

	// Per-cell: 2026-06-01|breakfast (165), 2026-06-01|dinner (65),
	// 2026-06-02|breakfast (165) — ordered by day then slot.
	if len(sl.ByCell) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(sl.ByCell))
	}
	want := []struct {
		date string
		slot string
		kcal float64
	}{
		{"2026-06-01", "breakfast", 165},
		{"2026-06-01", "dinner", 65},
		{"2026-06-02", "breakfast", 165},
	}
	for i, w := range want {
		c := sl.ByCell[i]
		if c.Date.Format("2006-01-02") != w.date || c.Slot != w.slot || !eq(c.Macros.Kcal, w.kcal) {
			t.Errorf("cell[%d] = %s|%s %.2f, want %s|%s %.2f",
				i, c.Date.Format("2006-01-02"), c.Slot, c.Macros.Kcal, w.date, w.slot, w.kcal)
		}
	}
}

func TestBuildShoppingListMarksIncomplete(t *testing.T) {
	lines := []line{
		{slot: "lunch", productID: 1, name: "Salt", grams: 5, kcal100: nil, protein100: nil, fat100: nil, carbs100: nil},
	}
	sl := buildShoppingList(lines)
	if sl.Totals.Complete {
		t.Error("expected incomplete totals when a product lacks macros")
	}
	if sl.Items[0].Macros.Complete {
		t.Error("expected incomplete item macros for product without nutrition")
	}
}

// A raw product placed directly into a plan cell (no recipe scaling) merges by
// product id with any recipe-derived need for the same product.
func TestBuildShoppingListMergesDirectProductLine(t *testing.T) {
	chickenKcal := ptr(165.0)
	d := mkDate("2026-06-01")
	lines := []line{
		// Recipe-derived chicken need.
		{slot: "lunch", date: d, productID: 1, name: "Chicken", grams: 100, kcal100: chickenKcal, protein100: ptr(31), fat100: ptr(3.6), carbs100: ptr(0)},
		// Direct product line for the same chicken, plus a product-only rice line.
		{slot: "dinner", date: d, productID: 1, name: "Chicken", grams: 150, kcal100: chickenKcal, protein100: ptr(31), fat100: ptr(3.6), carbs100: ptr(0)},
		{slot: "dinner", date: d, productID: 2, name: "Rice", grams: 50, kcal100: ptr(130), protein100: ptr(2.7), fat100: ptr(0.3), carbs100: ptr(28)},
	}

	sl := buildShoppingList(lines)

	if len(sl.Items) != 2 {
		t.Fatalf("expected 2 products, got %d", len(sl.Items))
	}
	if sl.Items[0].ProductName != "Chicken" || !eq(sl.Items[0].Grams, 250) {
		t.Errorf("chicken should merge to 250g, got %+v", sl.Items[0])
	}
	// Totals: chicken 250g (412.5 kcal) + rice 50g (65 kcal) = 477.5.
	if !eq(sl.Totals.Kcal, 477.5) {
		t.Errorf("total kcal = %.2f, want 477.5", sl.Totals.Kcal)
	}
}

func TestBuildShoppingListEmpty(t *testing.T) {
	sl := buildShoppingList(nil)
	if len(sl.Items) != 0 || len(sl.BySlot) != 0 || len(sl.ByDay) != 0 || len(sl.ByCell) != 0 {
		t.Errorf("expected empty list, got %+v", sl)
	}
	if !sl.Totals.Complete {
		t.Error("empty list should be complete")
	}
}

func eq(a, b float64) bool { return math.Abs(a-b) < 0.01 }
