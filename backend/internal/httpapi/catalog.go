package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/products"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
)

var validMealTypes = map[string]bool{
	"breakfast": true, "lunch": true, "dinner": true, "snack": true,
}

// --- products ---

type productRequest struct {
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Brand         string   `json:"brand"`
	Kcal100       *float64 `json:"kcal100"`
	Protein100    *float64 `json:"protein100"`
	Fat100        *float64 `json:"fat100"`
	Carbs100      *float64 `json:"carbs100"`
	Fiber100      *float64 `json:"fiber100"`
	GlycemicIndex *float64 `json:"glycemic_index"`
	Source        string   `json:"source"`
}

func (req productRequest) toProduct(userID, id int64) products.Product {
	return products.Product{
		ID: id, UserID: userID,
		Name: req.Name, Category: req.Category, Brand: req.Brand,
		Kcal100: req.Kcal100, Protein100: req.Protein100,
		Fat100: req.Fat100, Carbs100: req.Carbs100,
		Fiber100:      req.Fiber100,
		GlycemicIndex: req.GlycemicIndex, Source: req.Source,
	}
}

func (h *Handlers) createProduct(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req productRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	p, err := h.Products.Create(r.Context(), req.toProduct(userID, 0))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create product")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handlers) listProducts(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	list, err := h.Products.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list products")
		return
	}
	if list == nil {
		list = []products.Product{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) getProduct(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := h.Products.Get(r.Context(), userID, id)
	if errors.Is(err, products.ErrNotFound) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load product")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handlers) updateProduct(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req productRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	p, err := h.Products.Update(r.Context(), req.toProduct(userID, id))
	if errors.Is(err, products.ErrNotFound) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update product")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handlers) deleteProduct(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := h.Products.Delete(r.Context(), userID, id)
	if errors.Is(err, products.ErrNotFound) {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}
	if errors.Is(err, products.ErrInUse) {
		writeError(w, http.StatusConflict, "Продукт используется в рецептах — сначала удалите его оттуда")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete product")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- recipes ---

type recipeIngredientRequest struct {
	ProductID int64   `json:"product_id"`
	Grams     float64 `json:"grams"`
}

type recipeRequest struct {
	Name         string                    `json:"name"`
	Servings     int                       `json:"servings"`
	Instructions string                    `json:"instructions"`
	MealTypes    []string                  `json:"meal_types"`
	Source       string                    `json:"source"`
	Ingredients  []recipeIngredientRequest `json:"ingredients"`
}

func (req recipeRequest) validate() (string, bool) {
	if req.Name == "" {
		return "name is required", false
	}
	for _, mt := range req.MealTypes {
		if !validMealTypes[mt] {
			return "meal_types must be among breakfast/lunch/dinner/snack", false
		}
	}
	for _, in := range req.Ingredients {
		if in.ProductID <= 0 {
			return "each ingredient needs a product_id", false
		}
		if in.Grams <= 0 {
			return "ingredient grams must be positive", false
		}
	}
	return "", true
}

func (req recipeRequest) toRecipe(userID, id int64) recipes.Recipe {
	ings := make([]recipes.Ingredient, len(req.Ingredients))
	for i, in := range req.Ingredients {
		ings[i] = recipes.Ingredient{ProductID: in.ProductID, Grams: in.Grams}
	}
	return recipes.Recipe{
		ID: id, UserID: userID,
		Name: req.Name, Servings: req.Servings, Instructions: req.Instructions,
		MealTypes: req.MealTypes, Source: req.Source, Ingredients: ings,
	}
}

func (h *Handlers) createRecipe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req recipeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg, ok := req.validate(); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	rec, err := h.Recipes.Create(r.Context(), req.toRecipe(userID, 0))
	if errors.Is(err, recipes.ErrInvalidIngredient) {
		writeError(w, http.StatusBadRequest, "ingredient references unknown product")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create recipe")
		return
	}
	writeJSON(w, http.StatusCreated, recipeResponse(rec))
}

func (h *Handlers) listRecipes(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	list, err := h.Recipes.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list recipes")
		return
	}
	if list == nil {
		list = []recipes.Recipe{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) getRecipe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rec, err := h.Recipes.Get(r.Context(), userID, id)
	if errors.Is(err, recipes.ErrNotFound) {
		writeError(w, http.StatusNotFound, "recipe not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load recipe")
		return
	}
	writeJSON(w, http.StatusOK, recipeResponse(rec))
}

func (h *Handlers) updateRecipe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req recipeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg, ok := req.validate(); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	rec, err := h.Recipes.Update(r.Context(), req.toRecipe(userID, id))
	switch {
	case errors.Is(err, recipes.ErrNotFound):
		writeError(w, http.StatusNotFound, "recipe not found")
	case errors.Is(err, recipes.ErrInvalidIngredient):
		writeError(w, http.StatusBadRequest, "ingredient references unknown product")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not update recipe")
	default:
		writeJSON(w, http.StatusOK, recipeResponse(rec))
	}
}

func (h *Handlers) deleteRecipe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := h.Recipes.Delete(r.Context(), userID, id)
	if errors.Is(err, recipes.ErrNotFound) {
		writeError(w, http.StatusNotFound, "recipe not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete recipe")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// recipeResponse attaches computed total and per-serving macros to a recipe.
func recipeResponse(rec recipes.Recipe) map[string]any {
	total := rec.Macros()
	perServing := total
	if rec.Servings > 0 {
		s := float64(rec.Servings)
		perServing = recipes.Macros{
			Kcal: total.Kcal / s, Protein: total.Protein / s,
			Fat: total.Fat / s, Carbs: total.Carbs / s, Complete: total.Complete,
		}
	}
	return map[string]any{
		"recipe":             rec,
		"macros":             total,
		"macros_per_serving": perServing,
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}
