package nutrition

import (
	"errors"
	"math"
	"testing"
)

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.1 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}

func TestCalculate_MaleMaintain(t *testing.T) {
	// 80kg, 180cm, 30y, moderate (1.55), maintain.
	got, err := Calculate(Input{
		Sex: Male, WeightKg: 80, HeightCm: 180, Age: 30,
		Activity: Moderate, Goal: Maintain,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// BMR = 10*80 + 6.25*180 - 5*30 + 5 = 1780
	approx(t, "BMR", got.BMR, 1780)
	// TDEE = 1780 * 1.55 = 2759
	approx(t, "TDEE", got.TDEE, 2759)
	// maintain => calories = TDEE
	approx(t, "Calories", got.Calories, 2759)
	// protein = 1.8 * 80 = 144g
	approx(t, "ProteinG", got.ProteinG, 144)
	// fat = 2759 * 0.25 / 9 = 76.6389g
	approx(t, "FatG", got.FatG, 76.6389)
	// carbs = (2759 - 144*4 - 76.6389*9) / 4 = 373.3125g
	approx(t, "CarbsG", got.CarbsG, 373.3125)
}

func TestCalculate_FemaleLose(t *testing.T) {
	// 60kg, 165cm, 28y, light (1.375), lose (-20%).
	got, err := Calculate(Input{
		Sex: Female, WeightKg: 60, HeightCm: 165, Age: 28,
		Activity: Light, Goal: Lose,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// BMR = 600 + 1031.25 - 140 - 161 = 1330.25
	approx(t, "BMR", got.BMR, 1330.25)
	// TDEE = 1330.25 * 1.375 = 1829.09375
	approx(t, "TDEE", got.TDEE, 1829.09375)
	// lose => calories = TDEE * 0.8 = 1463.275
	approx(t, "Calories", got.Calories, 1463.275)
	approx(t, "ProteinG", got.ProteinG, 108)
}

func TestCalculate_WeightChangeRecomputesTargets(t *testing.T) {
	base := Input{Sex: Male, WeightKg: 80, HeightCm: 180, Age: 30, Activity: Moderate, Goal: Maintain}
	a, _ := Calculate(base)
	base.WeightKg = 85
	b, _ := Calculate(base)
	if b.Calories <= a.Calories {
		t.Errorf("expected higher calories for heavier weight: got %.2f then %.2f", a.Calories, b.Calories)
	}
	if b.ProteinG <= a.ProteinG {
		t.Errorf("expected more protein for heavier weight: got %.2f then %.2f", a.ProteinG, b.ProteinG)
	}
}

func TestCalculate_CustomMacros(t *testing.T) {
	got, err := Calculate(Input{
		Sex: Male, WeightKg: 80, HeightCm: 180, Age: 30,
		Activity: Moderate, Goal: Maintain,
		ProteinPerKg: 2.0, FatPct: 0.30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	approx(t, "ProteinG", got.ProteinG, 160)          // 2.0 * 80
	approx(t, "FatG", got.FatG, 2759*0.30/9)          // ~91.97
}

func TestCalculate_Invalid(t *testing.T) {
	cases := []struct {
		name string
		in   Input
	}{
		{"zero weight", Input{Sex: Male, WeightKg: 0, HeightCm: 180, Age: 30, Activity: Moderate, Goal: Maintain}},
		{"bad sex", Input{Sex: "other", WeightKg: 80, HeightCm: 180, Age: 30, Activity: Moderate, Goal: Maintain}},
		{"bad activity", Input{Sex: Male, WeightKg: 80, HeightCm: 180, Age: 30, Activity: "lazy", Goal: Maintain}},
		{"bad goal", Input{Sex: Male, WeightKg: 80, HeightCm: 180, Age: 30, Activity: Moderate, Goal: "bulk"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Calculate(c.in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}
