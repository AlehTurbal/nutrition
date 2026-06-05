// Package httpapi exposes the REST API: routing, request decoding and the
// HTTP handlers for auth, profile, weight and nutrition targets.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alehturbal/nutrition/backend/internal/assistant"
	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/chat"
	"github.com/alehturbal/nutrition/backend/internal/mealplans"
	"github.com/alehturbal/nutrition/backend/internal/nutrition"
	"github.com/alehturbal/nutrition/backend/internal/products"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/stores"
	"github.com/alehturbal/nutrition/backend/internal/users"
)

// StoreMatcher matches needed ingredients to store text (stores.Service in prod).
type StoreMatcher interface {
	Match(ctx context.Context, needed []stores.Needed, storeText string) ([]stores.Match, error)
}

// RecipeGenerator produces a recipe draft (recipes.Generator in prod).
type RecipeGenerator interface {
	Generate(ctx context.Context, description string, slots []string, servings int) (recipes.GeneratedRecipe, error)
}

// Handlers bundles the dependencies the HTTP layer needs.
type Handlers struct {
	Auth         *auth.Service
	Users        *users.Store
	Tokens       *auth.Manager
	Products     *products.Store
	Recipes      *recipes.Store
	MealPlans    *mealplans.Store
	StoreMatcher StoreMatcher
	StoreStore   *stores.Store
	RecipeGen    RecipeGenerator
	Chat         *chat.Store
	Assistant    *assistant.Assistant
}

// Router builds the chi router with all routes wired up.
func (h *Handlers) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.health)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(h.Tokens.Middleware)
			r.Get("/profile", h.getProfile)
			r.Put("/profile", h.putProfile)
			r.Post("/weights", h.addWeight)
			r.Get("/weights", h.listWeights)
			r.Get("/targets", h.getTargets)

			r.Route("/products", func(r chi.Router) {
				r.Post("/", h.createProduct)
				r.Get("/", h.listProducts)
				r.Get("/{id}", h.getProduct)
				r.Put("/{id}", h.updateProduct)
				r.Delete("/{id}", h.deleteProduct)
			})

			r.Route("/recipes", func(r chi.Router) {
				r.Post("/", h.createRecipe)
				r.Get("/", h.listRecipes)
				r.Get("/{id}", h.getRecipe)
				r.Put("/{id}", h.updateRecipe)
				r.Delete("/{id}", h.deleteRecipe)
			})

			r.Route("/meal-plans", func(r chi.Router) {
				r.Post("/", h.createPlan)
				r.Get("/", h.listPlans)
				r.Get("/{id}", h.getPlan)
				r.Delete("/{id}", h.deletePlan)
				r.Post("/{id}/items", h.addItem)
				r.Post("/{id}/copy-day", h.copyDay)
				r.Patch("/{id}/items/{itemID}", h.updateItem)
				r.Delete("/{id}/items/{itemID}", h.deleteItem)
				r.Get("/{id}/shopping-list", h.shoppingList)
			})

			r.Post("/stores/match", h.storeMatch)
			r.Get("/stores/matches/{planID}", h.getStoreMatches)
			r.Post("/recipes/generate", h.generateRecipe)

			r.Route("/chat/threads", func(r chi.Router) {
				r.Get("/", h.listThreads)
				r.Post("/", h.createThread)
				r.Put("/{id}", h.renameThread)
				r.Delete("/{id}", h.deleteThread)
				r.Get("/{id}/messages", h.listMessages)
				r.Post("/{id}/messages", h.postMessage)
			})
		})
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	token, err := h.Auth.Register(r.Context(), c.Email, c.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, users.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not register")
	default:
		writeJSON(w, http.StatusCreated, map[string]string{"token": token})
	}
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	token, err := h.Auth.Login(r.Context(), c.Email, c.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not log in")
	default:
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

func (h *Handlers) getProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	p, err := h.Users.GetProfile(r.Context(), userID)
	if errors.Is(err, users.ErrNotFound) {
		writeError(w, http.StatusNotFound, "profile not set")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load profile")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type profileRequest struct {
	Sex           string   `json:"sex"`
	HeightCm      float64  `json:"height_cm"`
	Age           int      `json:"age"`
	ActivityLevel string   `json:"activity_level"`
	Goal          string   `json:"goal"`
	ProteinPerKg  float64  `json:"protein_per_kg"`
	FatPct        float64  `json:"fat_pct"`
	MealSlots     []string `json:"meal_slots"`
}

func (h *Handlers) putProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg, ok := validateProfile(req); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p, err := h.Users.UpsertProfile(r.Context(), users.Profile{
		UserID:        userID,
		Sex:           req.Sex,
		HeightCm:      req.HeightCm,
		Age:           req.Age,
		ActivityLevel: req.ActivityLevel,
		Goal:          req.Goal,
		ProteinPerKg:  req.ProteinPerKg,
		FatPct:        req.FatPct,
		MealSlots:     req.MealSlots,
	})
	if err != nil {
		if errors.Is(err, users.ErrUserMissing) {
			writeError(w, http.StatusUnauthorized, "session no longer valid; please sign in again")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not save profile")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type weightRequest struct {
	WeightKg float64 `json:"weight_kg"`
}

func (h *Handlers) addWeight(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req weightRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.WeightKg <= 0 {
		writeError(w, http.StatusBadRequest, "weight_kg must be positive")
		return
	}
	entry, err := h.Users.AddWeight(r.Context(), userID, req.WeightKg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save weight")
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *Handlers) listWeights(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	list, err := h.Users.ListWeights(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load weights")
		return
	}
	if list == nil {
		list = []users.WeightEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) getTargets(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	p, err := h.Users.GetProfile(r.Context(), userID)
	if errors.Is(err, users.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "set your profile first")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load profile")
		return
	}

	weight, err := h.Users.LatestWeight(r.Context(), userID)
	if errors.Is(err, users.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "add a weight entry first")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load weight")
		return
	}

	targets, err := nutrition.Calculate(nutrition.Input{
		Sex:          nutrition.Sex(p.Sex),
		WeightKg:     weight.WeightKg,
		HeightCm:     p.HeightCm,
		Age:          p.Age,
		Activity:     nutrition.ActivityLevel(p.ActivityLevel),
		Goal:         nutrition.Goal(p.Goal),
		ProteinPerKg: p.ProteinPerKg,
		FatPct:       p.FatPct,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slots := p.MealSlots
	if len(slots) == 0 {
		slots = defaultMealSlots
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"weight_kg":  weight.WeightKg,
		"targets":    targets,
		"meal_slots": slots,
		"per_meal":   nutrition.SplitEven(targets, slots),
	})
}

// --- helpers ---

// defaultMealSlots is used when a profile has not configured its own.
var defaultMealSlots = []string{"breakfast", "lunch", "dinner"}

func validateProfile(req profileRequest) (string, bool) {
	switch nutrition.Sex(req.Sex) {
	case nutrition.Male, nutrition.Female:
	default:
		return "sex must be 'male' or 'female'", false
	}
	switch nutrition.ActivityLevel(req.ActivityLevel) {
	case nutrition.Sedentary, nutrition.Light, nutrition.Moderate, nutrition.Active, nutrition.VeryActive:
	default:
		return "invalid activity_level", false
	}
	switch nutrition.Goal(req.Goal) {
	case nutrition.Lose, nutrition.Maintain, nutrition.Gain:
	default:
		return "goal must be 'lose', 'maintain' or 'gain'", false
	}
	if req.HeightCm <= 0 || req.Age <= 0 {
		return "height_cm and age must be positive", false
	}
	for _, slot := range req.MealSlots {
		if !validMealTypes[slot] {
			return "meal_slots must be from: breakfast, lunch, dinner, snack", false
		}
	}
	return "", true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
