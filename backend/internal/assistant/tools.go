package assistant

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alehturbal/nutrition/backend/internal/llm"
)

// Tool names. Read tools are executed; propose_* tools become proposals.
const (
	toolListProducts   = "list_products"
	toolListRecipes    = "list_recipes"
	toolGetTargets     = "get_targets"
	toolProposeProduct = "propose_product"
	toolProposeRecipe  = "propose_recipe"
)

const systemPrompt = `Ты — ассистент приложения для питания. Помогаешь пользователю вести базу продуктов и рецептов и считать БЖУ.

Отвечай на том же языке, что и пользователь (обычно по-русски). Будь кратким.

Инструменты чтения (вызывай их, чтобы узнать текущие данные перед ответом):
- list_products — список продуктов пользователя (с их id и БЖУ на 100 г);
- list_recipes — список рецептов;
- get_targets — суточные цели КБЖУ (может вернуть ошибку, если не заполнен профиль или вес).

Инструменты изменения (НЕ выполняются сразу — становятся предложением, которое пользователь подтверждает кнопкой «Применить»):
- propose_product — предложить создать продукт (БЖУ на 100 г);
- propose_recipe — предложить создать рецепт. Ингредиенты ссылаются на product_id из list_products. ОБЯЗАТЕЛЬНО сначала вызови list_products, чтобы взять реальные id. Если нужного продукта нет, не выдумывай id: вместо этого попроси пользователя сначала создать продукт (или предложи его через propose_product).

Никогда не утверждай, что что-то создано: изменения применяет только пользователь.`

// toolDefs is the static tool set advertised to the model.
var toolDefs = []llm.Tool{
	{
		Name:        toolListProducts,
		Description: "Вернуть список продуктов пользователя с id и БЖУ на 100 г.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		Name:        toolListRecipes,
		Description: "Вернуть список рецептов пользователя.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		Name:        toolGetTargets,
		Description: "Вернуть суточные цели КБЖУ пользователя.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		Name:        toolProposeProduct,
		Description: "Предложить пользователю создать продукт. Не создаёт его — пользователь подтверждает.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":       map[string]any{"type": "string"},
				"category":   map[string]any{"type": "string"},
				"brand":      map[string]any{"type": "string"},
				"kcal100":    map[string]any{"type": "number"},
				"protein100": map[string]any{"type": "number"},
				"fat100":     map[string]any{"type": "number"},
				"carbs100":   map[string]any{"type": "number"},
				"source":     map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
	},
	{
		Name:        toolProposeRecipe,
		Description: "Предложить пользователю создать рецепт. Ингредиенты ссылаются на product_id из list_products.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string"},
				"servings":     map[string]any{"type": "integer"},
				"instructions": map[string]any{"type": "string"},
				"meal_types": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string", "enum": []string{"breakfast", "lunch", "dinner", "snack"}},
				},
				"ingredients": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"product_id": map[string]any{"type": "integer"},
							"grams":      map[string]any{"type": "number"},
						},
						"required": []string{"product_id", "grams"},
					},
				},
			},
			"required": []string{"name"},
		},
	},
}

// parseProposal converts a mutating tool call into a Proposal. It is pure: the
// raw tool input becomes the payload sent verbatim to the CRUD endpoint, after
// a light validity check. It never mutates anything.
func parseProposal(toolName string, input json.RawMessage) (Proposal, error) {
	var payload map[string]any
	if err := json.Unmarshal(input, &payload); err != nil {
		return Proposal{}, fmt.Errorf("invalid tool input: %w", err)
	}
	name, _ := payload["name"].(string)
	if name == "" {
		return Proposal{}, errors.New("proposal is missing a name")
	}

	switch toolName {
	case toolProposeProduct:
		return Proposal{Type: "product", Payload: payload}, nil
	case toolProposeRecipe:
		return Proposal{Type: "recipe", Payload: payload}, nil
	default:
		return Proposal{}, fmt.Errorf("%q is not a proposal tool", toolName)
	}
}
