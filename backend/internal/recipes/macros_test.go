package recipes

import (
	"math"
	"testing"
)

func ptr(f float64) *float64 { return &f }

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}

func TestComputeMacros_Complete(t *testing.T) {
	// 200g chicken (165kcal, 31p, 3.6f, 0c per 100g) + 150g rice (130, 2.7, 0.3, 28).
	m := ComputeMacros([]IngredientMacro{
		{Grams: 200, Kcal100: ptr(165), Protein100: ptr(31), Fat100: ptr(3.6), Carbs100: ptr(0)},
		{Grams: 150, Kcal100: ptr(130), Protein100: ptr(2.7), Fat100: ptr(0.3), Carbs100: ptr(28)},
	})
	if !m.Complete {
		t.Error("expected Complete=true")
	}
	approx(t, "Kcal", m.Kcal, 165*2+130*1.5)       // 330 + 195 = 525
	approx(t, "Protein", m.Protein, 31*2+2.7*1.5)  // 62 + 4.05 = 66.05
	approx(t, "Fat", m.Fat, 3.6*2+0.3*1.5)         // 7.2 + 0.45 = 7.65
	approx(t, "Carbs", m.Carbs, 0*2+28*1.5)        // 0 + 42 = 42
}

func TestComputeMacros_IncompleteWhenMissingBJU(t *testing.T) {
	m := ComputeMacros([]IngredientMacro{
		{Grams: 100, Kcal100: ptr(100), Protein100: ptr(10), Fat100: ptr(5), Carbs100: ptr(3)},
		{Grams: 100, Kcal100: nil, Protein100: nil, Fat100: nil, Carbs100: nil}, // unfilled product
	})
	if m.Complete {
		t.Error("expected Complete=false when an ingredient lacks БЖУ")
	}
	approx(t, "Kcal", m.Kcal, 100) // only the filled ingredient counts
}

func TestComputeMacros_Empty(t *testing.T) {
	m := ComputeMacros(nil)
	if !m.Complete {
		t.Error("empty recipe should be Complete")
	}
	approx(t, "Kcal", m.Kcal, 0)
}
