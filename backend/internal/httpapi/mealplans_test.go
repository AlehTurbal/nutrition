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

func TestAddProductItemToPlan(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "productplanner@example.com")

	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)

	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-02",
	})
	planID := int64(out["id"].(float64))
	items := srv.URL + "/api/meal-plans/" + itoa(planID) + "/items"

	// Add a raw product (150g chicken) directly into a lunch cell.
	resp, item := doJSON(t, http.MethodPost, items, token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "product_id": chicken, "grams": 150,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add product item: status %d, body %v", resp.StatusCode, item)
	}
	if item["product_name"].(string) != "Курица" {
		t.Errorf("product_name = %v, want Курица", item["product_name"])
	}
	approxEq(t, "item grams", item["grams"].(float64), 150)

	// Shopping list includes the product with its grams + kcal (165 * 1.5 = 247.5).
	resp, out = doJSON(t, http.MethodGet, srv.URL+"/api/meal-plans/"+itoa(planID)+"/shopping-list", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("shopping list: status %d, body %v", resp.StatusCode, out)
	}
	list := out["items"].([]any)
	if len(list) != 1 {
		t.Fatalf("expected 1 product, got %d", len(list))
	}
	approxEq(t, "chicken grams", list[0].(map[string]any)["grams"].(float64), 150)
	approxEq(t, "total kcal", out["totals"].(map[string]any)["kcal"].(float64), 247.5)

	// Rejections: both ids, and a foreign product id.
	resp, _ = doJSON(t, http.MethodPost, items, token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "recipe_id": 1, "product_id": chicken, "grams": 100,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 when both recipe_id and product_id set, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, items, token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "product_id": 999999, "grams": 100,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown product, got %d", resp.StatusCode)
	}
}

func TestUpdateItemQuantity(t *testing.T) {
	srv := newServer(t)
	token := registerUser(t, srv.URL, "updateitem@example.com")

	chicken := createProduct(t, srv.URL, token, "Курица", 165, 31, 3.6, 0)
	rice := createProduct(t, srv.URL, token, "Рис", 130, 2.7, 0.3, 28)
	_, out := doJSON(t, http.MethodPost, srv.URL+"/api/recipes", token, map[string]any{
		"name": "Курица с рисом", "servings": 2,
		"meal_types": []string{"lunch"},
		"ingredients": []map[string]any{
			{"product_id": chicken, "grams": 200},
			{"product_id": rice, "grams": 150},
		},
	})
	recipeID := int64(out["recipe"].(map[string]any)["id"].(float64))

	_, out = doJSON(t, http.MethodPost, srv.URL+"/api/meal-plans", token, map[string]any{
		"name": "P", "start_date": "2026-06-01", "end_date": "2026-06-02",
	})
	planID := int64(out["id"].(float64))
	base := srv.URL + "/api/meal-plans/" + itoa(planID)

	// A product item (150g chicken) and a recipe item (1 serving).
	_, prod := doJSON(t, http.MethodPost, base+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "lunch", "product_id": chicken, "grams": 150,
	})
	productItemID := int64(prod["id"].(float64))
	_, rec := doJSON(t, http.MethodPost, base+"/items", token, map[string]any{
		"day_date": "2026-06-01", "meal_slot": "dinner", "recipe_id": recipeID, "servings": 1,
	})
	recipeItemID := int64(rec["id"].(float64))

	// Update product grams 150 -> 250.
	resp, updated := doJSON(t, http.MethodPatch, base+"/items/"+itoa(productItemID), token, map[string]any{
		"grams": 250,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update grams: status %d, body %v", resp.StatusCode, updated)
	}
	approxEq(t, "updated grams", updated["grams"].(float64), 250)
	if updated["product_name"].(string) != "Курица" {
		t.Errorf("product_name = %v, want Курица", updated["product_name"])
	}

	// Update recipe servings 1 -> 3.
	resp, updated = doJSON(t, http.MethodPatch, base+"/items/"+itoa(recipeItemID), token, map[string]any{
		"servings": 3,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update servings: status %d, body %v", resp.StatusCode, updated)
	}
	approxEq(t, "updated servings", updated["servings"].(float64), 3)

	// Shopping list reflects the new quantities: 250g chicken (product) +
	// recipe at 3/2 of (200g chicken + 150g rice) = 300g chicken + 225g rice.
	// Chicken total = 250 + 300 = 550g; rice = 225g.
	_, out = doJSON(t, http.MethodGet, base+"/shopping-list", token, nil)
	byName := map[string]float64{}
	for _, it := range out["items"].([]any) {
		m := it.(map[string]any)
		byName[m["product_name"].(string)] = m["grams"].(float64)
	}
	approxEq(t, "chicken grams", byName["Курица"], 550)
	approxEq(t, "rice grams", byName["Рис"], 225)

	// Rejections: wrong-kind field, non-positive, both fields, neither field.
	resp, _ = doJSON(t, http.MethodPatch, base+"/items/"+itoa(productItemID), token, map[string]any{"servings": 2})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("servings on product item: expected 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPatch, base+"/items/"+itoa(recipeItemID), token, map[string]any{"grams": 100})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("grams on recipe item: expected 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPatch, base+"/items/"+itoa(productItemID), token, map[string]any{"grams": 0})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("non-positive grams: expected 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPatch, base+"/items/"+itoa(productItemID), token, map[string]any{"grams": 100, "servings": 2})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("both fields: expected 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPatch, base+"/items/"+itoa(productItemID), token, map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("neither field: expected 400, got %d", resp.StatusCode)
	}

	// Another user cannot update this item (404).
	other := registerUser(t, srv.URL, "updateitem-other@example.com")
	resp, _ = doJSON(t, http.MethodPatch, base+"/items/"+itoa(productItemID), other, map[string]any{"grams": 200})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("foreign user update: expected 404, got %d", resp.StatusCode)
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
