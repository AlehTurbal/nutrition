package httpapi

import (
	"errors"
	"net/http"

	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/llm"
	"github.com/alehturbal/nutrition/backend/internal/mealplans"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/stores"
)

// llm503 writes a 503 when the LLM is unconfigured; returns true if it handled err.
func llm503(w http.ResponseWriter, err error) bool {
	if errors.Is(err, llm.ErrNoAPIKey) {
		writeError(w, http.StatusServiceUnavailable, "LLM-функции недоступны: не задан ANTHROPIC_API_KEY")
		return true
	}
	return false
}

type storeMatchRequest struct {
	PlanID    int64  `json:"plan_id"`
	StoreText string `json:"store_text"`
}

func (h *Handlers) storeMatch(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req storeMatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreText == "" {
		writeError(w, http.StatusBadRequest, "store_text is required")
		return
	}

	sl, err := h.MealPlans.ShoppingList(r.Context(), userID, req.PlanID)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load plan needs")
		return
	}

	needed := make([]stores.Needed, len(sl.Items))
	for i, it := range sl.Items {
		needed[i] = stores.Needed{Name: it.ProductName, Grams: it.Grams}
	}

	matches, err := h.StoreMatcher.Match(r.Context(), needed, req.StoreText)
	if llm503(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "LLM matching failed")
		return
	}

	saved, err := h.StoreStore.Save(r.Context(), userID, req.PlanID, matches)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save match")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *Handlers) getStoreMatches(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathIDParam(w, r, "planID")
	if !ok {
		return
	}
	saved, err := h.StoreStore.Latest(r.Context(), userID, planID)
	if errors.Is(err, stores.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no saved match for this plan")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load match")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type generateRequest struct {
	Description string   `json:"description"`
	MealTypes   []string `json:"meal_types"`
	Servings    int      `json:"servings"`
}

func (h *Handlers) generateRecipe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req generateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}

	gen, err := h.RecipeGen.Generate(r.Context(), req.Description, req.MealTypes, req.Servings)
	if llm503(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "LLM generation failed")
		return
	}

	owned, err := h.Products.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load products")
		return
	}
	pl := make([]recipes.Product, len(owned))
	for i, p := range owned {
		pl[i] = recipes.Product{ID: p.ID, Name: p.Name}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recipe":      gen,
		"ingredients": recipes.MatchIngredientsToProducts(gen, pl),
	})
}
