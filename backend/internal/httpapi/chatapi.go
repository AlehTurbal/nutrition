package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/alehturbal/nutrition/backend/internal/assistant"
	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/chat"
	"github.com/alehturbal/nutrition/backend/internal/nutrition"
	"github.com/alehturbal/nutrition/backend/internal/products"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/users"
)

// StoreData adapts the domain stores to assistant.DataSource, backing the
// assistant's read tools with the user's real data.
type StoreData struct {
	users    *users.Store
	products *products.Store
	recipes  *recipes.Store
}

// NewStoreData builds the assistant's read-tool data source.
func NewStoreData(u *users.Store, p *products.Store, r *recipes.Store) *StoreData {
	return &StoreData{users: u, products: p, recipes: r}
}

func (d *StoreData) ListProducts(ctx context.Context, userID int64) (any, error) {
	return d.products.List(ctx, userID)
}

func (d *StoreData) ListRecipes(ctx context.Context, userID int64) (any, error) {
	return d.recipes.List(ctx, userID)
}

// GetTargets mirrors the /api/targets computation so the assistant sees the same
// daily targets. A user-facing error (e.g. missing profile) is returned as a
// plain error and surfaced to the model.
func (d *StoreData) GetTargets(ctx context.Context, userID int64) (any, error) {
	p, err := d.users.GetProfile(ctx, userID)
	if errors.Is(err, users.ErrNotFound) {
		return nil, errors.New("профиль не заполнен")
	}
	if err != nil {
		return nil, err
	}
	weight, err := d.users.LatestWeight(ctx, userID)
	if errors.Is(err, users.ErrNotFound) {
		return nil, errors.New("нет записей веса")
	}
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return map[string]any{"weight_kg": weight.WeightKg, "targets": targets}, nil
}

// --- handlers ---

type chatThreadRequest struct {
	Title string `json:"title"`
}

func (h *Handlers) listThreads(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	threads, err := h.Chat.ListThreads(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load threads")
		return
	}
	if threads == nil {
		threads = []chat.Thread{}
	}
	writeJSON(w, http.StatusOK, threads)
}

func (h *Handlers) createThread(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req chatThreadRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	t, err := h.Chat.CreateThread(r.Context(), userID, req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create thread")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handlers) deleteThread(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathIDParam(w, r, "id")
	if !ok {
		return
	}
	err := h.Chat.DeleteThread(r.Context(), userID, id)
	if errors.Is(err, chat.ErrNotFound) {
		writeError(w, http.StatusNotFound, "thread not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete thread")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) listMessages(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.Chat.GetThread(r.Context(), userID, id); errors.Is(err, chat.ErrNotFound) {
		writeError(w, http.StatusNotFound, "thread not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load thread")
		return
	}
	msgs, err := h.Chat.ListMessages(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load messages")
		return
	}
	if msgs == nil {
		msgs = []chat.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

type chatMessageRequest struct {
	Content string `json:"content"`
}

func (h *Handlers) postMessage(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, ok := pathIDParam(w, r, "id")
	if !ok {
		return
	}
	var req chatMessageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	if _, err := h.Chat.GetThread(r.Context(), userID, id); errors.Is(err, chat.ErrNotFound) {
		writeError(w, http.StatusNotFound, "thread not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load thread")
		return
	}

	// Load prior dialog (without the new message) as history.
	prior, err := h.Chat.ListMessages(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load history")
		return
	}
	history := make([]assistant.Turn, len(prior))
	for i, m := range prior {
		history[i] = assistant.Turn{Role: m.Role, Text: m.Content}
	}

	// Run the assistant before persisting anything so a failure leaves no
	// orphan user message.
	text, proposals, err := h.Assistant.Reply(r.Context(), userID, history, req.Content)
	if llm503(w, err) {
		return
	}
	if errors.Is(err, assistant.ErrMaxIterations) {
		writeError(w, http.StatusBadGateway, "ассистент не смог завершить ответ")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "сбой ассистента")
		return
	}

	if _, err := h.Chat.AddMessage(r.Context(), id, "user", req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save message")
		return
	}
	saved, err := h.Chat.AddMessage(r.Context(), id, "assistant", text)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save reply")
		return
	}

	if proposals == nil {
		proposals = []assistant.Proposal{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":   saved,
		"proposals": proposals,
	})
}
