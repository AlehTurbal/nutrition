package httpapi_test

import (
	"net/http"
	"testing"
)

func TestMealPlanFlowAndShoppingList(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "planner@example.com")

	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)
	rice := createProduct(t, srv.URL, token, "Рис", 130, 2.7, 0.3, 28)

	// Recipe: 200g chicken + 150g rice, 2 servings.
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Курица с рисом", "servings": 2,
		"meal_types": []string{"lunch", "dinner"},
		"ingredients": []map[string]any{
			{"product_id": chicken, "grams": 200},
			{"product_id": rice, "grams": 150},
		},
	})
	recipeID := int64(out["recipe"].(map[string]any)["id"].(float64))

	// Create a one-week plan.
	resp, out := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "Неделя 1", "start_date": "2026-06-01", "end_date": "2026-06-07",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %v", resp.StatusCode, out)
	}
	planID := int64(out["id"].(float64))

	// Add two items: lunch (1 serving) and dinner (2 servings) -> 1.5x the recipe total.
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans/"+itoa(planID)+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "recipe_id": recipeID, "servings": 1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add lunch item: status %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans/"+itoa(planID)+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "dinner", "recipe_id": recipeID, "servings": 2,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add dinner item: status %d", resp.StatusCode)
	}

	// Shopping list aggregates scaled grams.
	// lunch: 1/2 of recipe -> 100g chicken + 75g rice.
	// dinner: 2/2 of recipe -> 200g chicken + 150g rice.
	// total: 300g chicken + 225g rice.
	resp, out = doJSON(t, http.MethodGet, srv.URL+"/api/meal-plans/"+itoa(planID)+"/shopping-list", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("shopping list: status %d, body %v", resp.StatusCode, out)
	}
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 products, got %d", len(items))
	}
	byName := map[string]float64{}
	for _, it := range items {
		m := it.(map[string]any)
		byName[m["product_name"].(string)] = m["grams"].(float64)
	}
	approxEq(t, "chicken grams", byName["Курица"], 300)
	approxEq(t, "rice grams", byName["Рис"], 225)

	totals := out["totals"].(map[string]any)
	// 300g chicken kcal = 495; 225g rice kcal = 292.5 -> 787.5
	approxEq(t, "total kcal", totals["kcal"].(float64), 787.5)
	if !totals["complete"].(bool) {
		t.Error("expected complete totals")
	}

	bySlot := out["by_slot"].([]any)
	if len(bySlot) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(bySlot))
	}
}

func TestCopyDayReplacesTargetDays(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "copyday@example.com")

	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)
	rice := createProduct(t, srv.URL, token, "Рис", 130, 2.7, 0.3, 28)
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Курица с рисом", "servings": 2,
		"meal_types": []string{"lunch", "dinner"},
		"ingredients": []map[string]any{
			{"product_id": chicken, "grams": 200},
			{"product_id": rice, "grams": 150},
		},
	})
	recipeID := int64(out["recipe"].(map[string]any)["id"].(float64))

	_, out = doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "Неделя 1", "start_date": "2026-06-01", "end_date": "2026-06-07",
	})
	planID := int64(out["id"].(float64))
	base := srv.URL + "/api/meal-plans/" + itoa(planID)

	// Source day 06-01: lunch + dinner.
	doJSON(t, http.MethodPost, base+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "recipe_id": recipeID, "servings": 1,
	})
	doJSON(t, http.MethodPost, base+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "dinner", "recipe_id": recipeID, "servings": 2,
	})
	// 06-03 already has a breakfast item that must be replaced away.
	doJSON(t, http.MethodPost, base+"/items", token, map[string]any{
		"day_date": "2026-06-03", "meal_slot": "breakfast", "recipe_id": recipeID, "servings": 1,
	})

	// Copy 06-01 onto 06-02 and 06-03.
	resp, _ := doJSON(t, http.MethodPost, base+"/copy-day", token, map[string]any{
		"source_date":  "2026-06-01",
		"target_dates": []string{"2026-06-02", "2026-06-03"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("copy-day: status %d", resp.StatusCode)
	}

	_, out = doJSON(t, http.MethodGet, base, token, nil)
	bySlotByDay := map[string][]string{}
	for _, raw := range out["items"].([]any) {
		it := raw.(map[string]any)
		day := it["day_date"].(string)
		bySlotByDay[day] = append(bySlotByDay[day], it["meal_slot"].(string))
	}
	for _, day := range []string{"2026-06-01", "2026-06-02", "2026-06-03"} {
		if len(bySlotByDay[day]) != 2 {
			t.Errorf("%s: expected 2 items (lunch+dinner), got %v", day, bySlotByDay[day])
		}
	}
	// The pre-existing breakfast on 06-03 must be gone (replace, not append).
	for _, slot := range bySlotByDay["2026-06-03"] {
		if slot == "breakfast" {
			t.Errorf("06-03 still has the replaced breakfast item")
		}
	}
}

func TestCopyDayRejectsOutOfRange(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "copyrange@example.com")
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-03",
	})
	planID := int64(out["id"].(float64))
	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans/"+itoa(planID)+"/copy-day", token, map[string]any{
		"source_date": "2026-06-01", "target_dates": []string{"2026-06-09"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range target, got %d", resp.StatusCode)
	}
}

func TestMealPlanRejectsForeignRecipe(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "planowner@example.com")

	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-02",
	})
	planID := int64(out["id"].(float64))

	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans/"+itoa(planID)+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "recipe_id": 999999, "servings": 1,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown recipe, got %d", resp.StatusCode)
	}
}

func TestTargetsIncludePerMealSplit(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "splituser@example.com")

	// Profile with 4 meal slots.
	resp, _ := doJSON(t, http.MethodPut, srv.URL+"/api/profile", token, map[string]any{
		"sex": "male", "height_cm": 180, "age": 30,
		"activity_level": "moderate", "goal": "maintain",
		"meal_slots": []string{"breakfast", "lunch", "dinner", "snack"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put profile: status %d", resp.StatusCode)
	}
	doJSON(t, http.MethodPost, srv.URL+"/api/weights", token, map[string]any{"weight_kg": 80})

	_, out := doJSON(t, http.MethodGet, srv.URL+"/api/targets", token, nil)
	perMeal := out["per_meal"].([]any)
	if len(perMeal) != 4 {
		t.Fatalf("expected 4 per-meal entries, got %d", len(perMeal))
	}
	dayCal := out["targets"].(map[string]any)["calories"].(float64)
	var sum float64
	for _, p := range perMeal {
		sum += p.(map[string]any)["calories"].(float64)
	}
	approxEq(t, "per-meal sum vs day", sum, dayCal)
}
