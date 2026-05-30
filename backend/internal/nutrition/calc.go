// Package nutrition computes daily calorie and macronutrient (БЖУ) targets from a
// user's body profile using the Mifflin–St Jeor equation.
package nutrition

import (
	"errors"
	"fmt"
)

// Sex of the user, required by the Mifflin–St Jeor BMR equation.
type Sex string

const (
	Male   Sex = "male"
	Female Sex = "female"
)

// ActivityLevel maps to a TDEE multiplier.
type ActivityLevel string

const (
	Sedentary  ActivityLevel = "sedentary"
	Light      ActivityLevel = "light"
	Moderate   ActivityLevel = "moderate"
	Active     ActivityLevel = "active"
	VeryActive ActivityLevel = "very_active"
)

// Goal adjusts calories relative to maintenance (TDEE).
type Goal string

const (
	Lose     Goal = "lose"
	Maintain Goal = "maintain"
	Gain     Goal = "gain"
)

// Defaults for macro distribution. Overridable per profile.
const (
	DefaultProteinPerKg = 1.8  // grams of protein per kg of body weight
	DefaultFatPct       = 0.25 // fraction of total calories from fat

	kcalPerGramProtein = 4.0
	kcalPerGramFat     = 9.0
	kcalPerGramCarbs   = 4.0
)

var activityFactors = map[ActivityLevel]float64{
	Sedentary:  1.2,
	Light:      1.375,
	Moderate:   1.55,
	Active:     1.725,
	VeryActive: 1.9,
}

var goalAdjustments = map[Goal]float64{
	Lose:     -0.20,
	Maintain: 0.0,
	Gain:     0.12,
}

// Input holds everything needed to compute targets. ProteinPerKg and FatPct are
// optional; zero values fall back to the package defaults.
type Input struct {
	Sex          Sex
	WeightKg     float64
	HeightCm     float64
	Age          int
	Activity     ActivityLevel
	Goal         Goal
	ProteinPerKg float64
	FatPct       float64
}

// Targets is the computed daily plan.
type Targets struct {
	BMR      float64 `json:"bmr"`
	TDEE     float64 `json:"tdee"`
	Calories float64 `json:"calories"`
	ProteinG float64 `json:"protein_g"`
	FatG     float64 `json:"fat_g"`
	CarbsG   float64 `json:"carbs_g"`
}

// ErrInvalidInput is returned when the profile cannot produce a sensible target.
var ErrInvalidInput = errors.New("invalid nutrition input")

// Calculate returns daily calorie and macro targets for the given profile.
func Calculate(in Input) (Targets, error) {
	if in.WeightKg <= 0 || in.HeightCm <= 0 || in.Age <= 0 {
		return Targets{}, fmt.Errorf("%w: weight, height and age must be positive", ErrInvalidInput)
	}

	bmr, err := bmr(in.Sex, in.WeightKg, in.HeightCm, in.Age)
	if err != nil {
		return Targets{}, err
	}

	factor, ok := activityFactors[in.Activity]
	if !ok {
		return Targets{}, fmt.Errorf("%w: unknown activity level %q", ErrInvalidInput, in.Activity)
	}

	adj, ok := goalAdjustments[in.Goal]
	if !ok {
		return Targets{}, fmt.Errorf("%w: unknown goal %q", ErrInvalidInput, in.Goal)
	}

	proteinPerKg := in.ProteinPerKg
	if proteinPerKg <= 0 {
		proteinPerKg = DefaultProteinPerKg
	}
	fatPct := in.FatPct
	if fatPct <= 0 {
		fatPct = DefaultFatPct
	}

	tdee := bmr * factor
	calories := tdee * (1 + adj)

	proteinG := proteinPerKg * in.WeightKg
	fatG := (calories * fatPct) / kcalPerGramFat
	carbsKcal := calories - proteinG*kcalPerGramProtein - fatG*kcalPerGramFat
	if carbsKcal < 0 {
		carbsKcal = 0
	}
	carbsG := carbsKcal / kcalPerGramCarbs

	return Targets{
		BMR:      bmr,
		TDEE:     tdee,
		Calories: calories,
		ProteinG: proteinG,
		FatG:     fatG,
		CarbsG:   carbsG,
	}, nil
}

func bmr(sex Sex, weightKg, heightCm float64, age int) (float64, error) {
	base := 10*weightKg + 6.25*heightCm - 5*float64(age)
	switch sex {
	case Male:
		return base + 5, nil
	case Female:
		return base - 161, nil
	default:
		return 0, fmt.Errorf("%w: unknown sex %q", ErrInvalidInput, sex)
	}
}
