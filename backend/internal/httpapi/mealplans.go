package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/mealplans"
)

const dateLayout = "2006-01-02"

type planRequest struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (h *Handlers) createPlan(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req planRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	start, ok := parseDate(w, req.StartDate, "start_date")
	if !ok {
		return
	}
	end, ok := parseDate(w, req.EndDate, "end_date")
	if !ok {
		return
	}
	if end.Before(start) {
		writeError(w, http.StatusBadRequest, "end_date must not be before start_date")
		return
	}
	p, err := h.MealPlans.CreatePlan(r.Context(), mealplans.Plan{
		UserID:    userID,
		Name:      req.Name,
		StartDate: mealplans.Date{Time: start},
		EndDate:   mealplans.Date{Time: end},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create plan")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handlers) listPlans(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	list, err := h.MealPlans.ListPlans(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list plans")
		return
	}
	if list == nil {
		list = []mealplans.Plan{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) getPlan(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := h.MealPlans.GetPlan(r.Context(), userID, id)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load plan")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handlers) deletePlan(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := h.MealPlans.DeletePlan(r.Context(), userID, id)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete plan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type itemRequest struct {
	DayDate   string   `json:"day_date"`
	MealSlot  string   `json:"meal_slot"`
	RecipeID  int64    `json:"recipe_id"`
	Servings  float64  `json:"servings"`
	ProductID *int64   `json:"product_id"`
	Grams     *float64 `json:"grams"`
}

func (h *Handlers) addItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req itemRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	day, ok := parseDate(w, req.DayDate, "day_date")
	if !ok {
		return
	}
	if req.MealSlot == "" {
		writeError(w, http.StatusBadRequest, "meal_slot is required")
		return
	}
	hasProduct := req.ProductID != nil
	hasRecipe := req.RecipeID != 0
	if hasProduct == hasRecipe {
		writeError(w, http.StatusBadRequest, "exactly one of recipe_id or product_id is required")
		return
	}
	if hasProduct && (req.Grams == nil || *req.Grams <= 0) {
		writeError(w, http.StatusBadRequest, "grams must be positive for a product item")
		return
	}
	it, err := h.MealPlans.AddItem(r.Context(), userID, mealplans.Item{
		MealPlanID: planID,
		DayDate:    mealplans.Date{Time: day},
		MealSlot:   req.MealSlot,
		RecipeID:   req.RecipeID,
		Servings:   req.Servings,
		ProductID:  req.ProductID,
		Grams:      req.Grams,
	})
	switch {
	case errors.Is(err, mealplans.ErrNotFound):
		writeError(w, http.StatusNotFound, "plan not found")
	case errors.Is(err, mealplans.ErrInvalidRecipe):
		writeError(w, http.StatusBadRequest, "recipe not found")
	case errors.Is(err, mealplans.ErrInvalidProduct):
		writeError(w, http.StatusBadRequest, "product not found")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not add item")
	default:
		writeJSON(w, http.StatusCreated, it)
	}
}

type copyDayRequest struct {
	SourceDate  string   `json:"source_date"`
	TargetDates []string `json:"target_dates"`
}

// copyDay replaces the items of each target day with copies of the source day's
// items. All dates must fall within the plan's range.
func (h *Handlers) copyDay(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req copyDayRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	source, ok := parseDate(w, req.SourceDate, "source_date")
	if !ok {
		return
	}
	if len(req.TargetDates) == 0 {
		writeError(w, http.StatusBadRequest, "target_dates is required")
		return
	}

	plan, err := h.MealPlans.GetPlan(r.Context(), userID, planID)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load plan")
		return
	}
	if !withinPlan(plan, source) {
		writeError(w, http.StatusBadRequest, "source_date is outside the plan range")
		return
	}

	targets := make([]mealplans.Date, 0, len(req.TargetDates))
	for _, s := range req.TargetDates {
		t, ok := parseDate(w, s, "target_dates")
		if !ok {
			return
		}
		if !withinPlan(plan, t) {
			writeError(w, http.StatusBadRequest, "target_dates contains a date outside the plan range")
			return
		}
		targets = append(targets, mealplans.Date{Time: t})
	}

	items, err := h.MealPlans.CopyDay(r.Context(), userID, planID, mealplans.Date{Time: source}, targets)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not copy day")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// withinPlan reports whether d falls within [StartDate, EndDate] inclusive.
func withinPlan(p mealplans.Plan, d time.Time) bool {
	return !d.Before(p.StartDate.Time) && !d.After(p.EndDate.Time)
}

type updateItemRequest struct {
	Servings *float64 `json:"servings"`
	Grams    *float64 `json:"grams"`
}

// updateItem changes the quantity of one plan item — servings for a recipe item
// or grams for a product item. Exactly one positive value must be supplied.
func (h *Handlers) updateItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathIDParam(w, r, "id")
	if !ok {
		return
	}
	itemID, ok := pathIDParam(w, r, "itemID")
	if !ok {
		return
	}
	var req updateItemRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if (req.Servings == nil) == (req.Grams == nil) {
		writeError(w, http.StatusBadRequest, "exactly one of servings or grams is required")
		return
	}
	if req.Servings != nil && *req.Servings <= 0 {
		writeError(w, http.StatusBadRequest, "servings must be positive")
		return
	}
	if req.Grams != nil && *req.Grams <= 0 {
		writeError(w, http.StatusBadRequest, "grams must be positive")
		return
	}

	it, err := h.MealPlans.UpdateItem(r.Context(), userID, planID, itemID, req.Servings, req.Grams)
	switch {
	case errors.Is(err, mealplans.ErrNotFound):
		writeError(w, http.StatusNotFound, "item not found")
	case errors.Is(err, mealplans.ErrInvalidProduct):
		writeError(w, http.StatusBadRequest, "grams can only be set on a product item")
	case errors.Is(err, mealplans.ErrInvalidRecipe):
		writeError(w, http.StatusBadRequest, "servings can only be set on a recipe item")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not update item")
	default:
		writeJSON(w, http.StatusOK, it)
	}
}

func (h *Handlers) deleteItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathIDParam(w, r, "id")
	if !ok {
		return
	}
	itemID, ok := pathIDParam(w, r, "itemID")
	if !ok {
		return
	}
	err := h.MealPlans.DeleteItem(r.Context(), userID, planID, itemID)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) shoppingList(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	planID, ok := pathID(w, r)
	if !ok {
		return
	}
	sl, err := h.MealPlans.ShoppingList(r.Context(), userID, planID)
	if errors.Is(err, mealplans.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not build shopping list")
		return
	}
	writeJSON(w, http.StatusOK, sl)
}

// pathIDParam parses a named int64 URL parameter.
func pathIDParam(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

func parseDate(w http.ResponseWriter, s, field string) (time.Time, bool) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		writeError(w, http.StatusBadRequest, field+" must be YYYY-MM-DD")
		return time.Time{}, false
	}
	return t, true
}
