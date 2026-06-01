package mealplans

import (
	"context"
	"fmt"
	"sort"

	"github.com/alehturbal/nutrition/backend/internal/recipes"
)

// ShoppingItem is the aggregated amount of one product needed across a plan,
// with the macros that amount contributes.
type ShoppingItem struct {
	ProductID   int64          `json:"product_id"`
	ProductName string         `json:"product_name"`
	Grams       float64        `json:"grams"`
	Macros      recipes.Macros `json:"macros"`
}

// SlotMacros is the total БЖУ planned for one meal slot across all days.
type SlotMacros struct {
	Slot   string         `json:"slot"`
	Macros recipes.Macros `json:"macros"`
}

// DayMacros is the total БЖУ planned for one day across all meal slots.
type DayMacros struct {
	Date   Date           `json:"date"`
	Macros recipes.Macros `json:"macros"`
}

// ShoppingList is the derived buy-list plus macro roll-ups for a plan.
type ShoppingList struct {
	Items  []ShoppingItem `json:"items"`
	Totals recipes.Macros `json:"totals"`
	BySlot []SlotMacros   `json:"by_slot"`
	ByDay  []DayMacros    `json:"by_day"`
}

// line is one ingredient occurrence scaled by the item's servings.
type line struct {
	slot       string
	date       Date
	productID  int64
	name       string
	grams      float64
	kcal100    *float64
	protein100 *float64
	fat100     *float64
	carbs100   *float64
}

// ShoppingList aggregates the ingredients required by every item in a plan.
// Each ingredient's grams are scaled by item.servings / recipe.servings so the
// amount reflects how much of the dish is actually planned. Returns ErrNotFound
// if the plan does not belong to the user.
func (s *Store) ShoppingList(ctx context.Context, userID, planID int64) (ShoppingList, error) {
	var owns bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM meal_plans WHERE id = $1 AND user_id = $2)`,
		planID, userID).Scan(&owns); err != nil {
		return ShoppingList{}, fmt.Errorf("verify plan: %w", err)
	}
	if !owns {
		return ShoppingList{}, ErrNotFound
	}

	rows, err := s.pool.Query(ctx,
		`SELECT mpi.meal_slot, mpi.day_date, ri.product_id, p.name,
		        ri.grams * mpi.servings / r.servings AS grams,
		        p.kcal100, p.protein100, p.fat100, p.carbs100
		 FROM meal_plan_items mpi
		 JOIN recipes r ON r.id = mpi.recipe_id
		 JOIN recipe_ingredients ri ON ri.recipe_id = r.id
		 JOIN products p ON p.id = ri.product_id
		 WHERE mpi.meal_plan_id = $1`, planID)
	if err != nil {
		return ShoppingList{}, fmt.Errorf("aggregate ingredients: %w", err)
	}
	defer rows.Close()

	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.slot, &l.date.Time, &l.productID, &l.name, &l.grams,
			&l.kcal100, &l.protein100, &l.fat100, &l.carbs100); err != nil {
			return ShoppingList{}, fmt.Errorf("scan line: %w", err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return ShoppingList{}, err
	}

	return buildShoppingList(lines), nil
}

// buildShoppingList rolls ingredient lines up into per-product totals, an overall
// total and a per-slot breakdown. Pure function for easy unit testing.
func buildShoppingList(lines []line) ShoppingList {
	type agg struct {
		name       string
		grams      float64
		kcal100    *float64
		protein100 *float64
		fat100     *float64
		carbs100   *float64
	}
	byProduct := map[int64]*agg{}
	order := []int64{}
	slotGrams := map[string][]recipes.IngredientMacro{}
	slotOrder := []string{}
	dayGrams := map[string][]recipes.IngredientMacro{}
	dayDates := map[string]Date{}
	dayOrder := []string{}

	for _, l := range lines {
		a, ok := byProduct[l.productID]
		if !ok {
			a = &agg{name: l.name, kcal100: l.kcal100, protein100: l.protein100, fat100: l.fat100, carbs100: l.carbs100}
			byProduct[l.productID] = a
			order = append(order, l.productID)
		}
		a.grams += l.grams

		if _, ok := slotGrams[l.slot]; !ok {
			slotOrder = append(slotOrder, l.slot)
		}
		slotGrams[l.slot] = append(slotGrams[l.slot], recipes.IngredientMacro{
			Grams: l.grams, Kcal100: l.kcal100, Protein100: l.protein100, Fat100: l.fat100, Carbs100: l.carbs100,
		})

		day := l.date.Format("2006-01-02")
		if _, ok := dayGrams[day]; !ok {
			dayOrder = append(dayOrder, day)
			dayDates[day] = l.date
		}
		dayGrams[day] = append(dayGrams[day], recipes.IngredientMacro{
			Grams: l.grams, Kcal100: l.kcal100, Protein100: l.protein100, Fat100: l.fat100, Carbs100: l.carbs100,
		})
	}

	sort.Slice(order, func(i, j int) bool {
		return byProduct[order[i]].name < byProduct[order[j]].name
	})

	out := ShoppingList{Items: []ShoppingItem{}, BySlot: []SlotMacros{}, ByDay: []DayMacros{}}
	var totalLines []recipes.IngredientMacro
	for _, id := range order {
		a := byProduct[id]
		m := recipes.ComputeMacros([]recipes.IngredientMacro{{
			Grams: a.grams, Kcal100: a.kcal100, Protein100: a.protein100, Fat100: a.fat100, Carbs100: a.carbs100,
		}})
		out.Items = append(out.Items, ShoppingItem{
			ProductID: id, ProductName: a.name, Grams: a.grams, Macros: m,
		})
		totalLines = append(totalLines, recipes.IngredientMacro{
			Grams: a.grams, Kcal100: a.kcal100, Protein100: a.protein100, Fat100: a.fat100, Carbs100: a.carbs100,
		})
	}
	out.Totals = recipes.ComputeMacros(totalLines)

	sort.Strings(slotOrder)
	for _, slot := range slotOrder {
		out.BySlot = append(out.BySlot, SlotMacros{Slot: slot, Macros: recipes.ComputeMacros(slotGrams[slot])})
	}

	sort.Strings(dayOrder)
	for _, day := range dayOrder {
		out.ByDay = append(out.ByDay, DayMacros{Date: dayDates[day], Macros: recipes.ComputeMacros(dayGrams[day])})
	}
	return out
}
