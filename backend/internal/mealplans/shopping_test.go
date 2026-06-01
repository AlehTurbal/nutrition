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

func TestBuildShoppingListEmpty(t *testing.T) {
	sl := buildShoppingList(nil)
	if len(sl.Items) != 0 || len(sl.BySlot) != 0 || len(sl.ByDay) != 0 {
		t.Errorf("expected empty list, got %+v", sl)
	}
	if !sl.Totals.Complete {
		t.Error("empty list should be complete")
	}
}

func eq(a, b float64) bool { return math.Abs(a-b) < 0.01 }
