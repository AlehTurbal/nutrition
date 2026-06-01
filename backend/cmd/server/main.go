// Command server runs the nutrition REST API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alehturbal/nutrition/backend/internal/assistant"
	"github.com/alehturbal/nutrition/backend/internal/auth"
	"github.com/alehturbal/nutrition/backend/internal/chat"
	"github.com/alehturbal/nutrition/backend/internal/config"
	"github.com/alehturbal/nutrition/backend/internal/db"
	"github.com/alehturbal/nutrition/backend/internal/httpapi"
	"github.com/alehturbal/nutrition/backend/internal/llm"
	"github.com/alehturbal/nutrition/backend/internal/mealplans"
	"github.com/alehturbal/nutrition/backend/internal/products"
	"github.com/alehturbal/nutrition/backend/internal/recipes"
	"github.com/alehturbal/nutrition/backend/internal/stores"
	"github.com/alehturbal/nutrition/backend/internal/users"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	store := users.NewStore(pool)
	tokens := auth.NewManager(cfg.JWTSecret, cfg.JWTTTL)
	authSvc := auth.NewService(store, tokens)

	llmClient := llm.New(cfg.AnthropicAPIKey, cfg.LLMModel)

	productStore := products.NewStore(pool)
	recipeStore := recipes.NewStore(pool)

	handlers := &httpapi.Handlers{
		Auth:         authSvc,
		Users:        store,
		Tokens:       tokens,
		Products:     productStore,
		Recipes:      recipeStore,
		MealPlans:    mealplans.NewStore(pool),
		StoreMatcher: stores.NewService(llmClient),
		StoreStore:   stores.NewStore(pool),
		RecipeGen:    recipes.NewGenerator(llmClient),
		Chat:         chat.NewStore(pool),
		Assistant:    assistant.New(llmClient, httpapi.NewStoreData(store, productStore, recipeStore)),
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handlers.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
