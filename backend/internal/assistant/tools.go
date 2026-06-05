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
	toolListMealPlans  = "list_meal_plans"
	toolProposeProduct   = "propose_product"
	toolProposeRecipe    = "propose_recipe"
	toolProposeCopyDay   = "propose_copy_day"
	toolProposeAddToPlan = "propose_add_to_plan"
)

const systemPrompt = `Ты — ассистент приложения для питания. Помогаешь пользователю вести базу продуктов и рецептов и считать БЖУ.

Отвечай на том же языке, что и пользователь (обычно по-русски). Будь кратким.

Инструменты чтения (вызывай их, чтобы узнать текущие данные перед ответом):
- list_products — список продуктов пользователя (с их id и БЖУ на 100 г);
- list_recipes — список рецептов;
- get_targets — суточные цели КБЖУ (может вернуть ошибку, если не заполнен профиль или вес);
- list_meal_plans — список планов питания пользователя с их id, диапазоном дат и блюдами по дням (нужно, чтобы скопировать день).

Инструменты изменения (НЕ выполняются сразу — становятся предложением, которое пользователь подтверждает кнопкой «Применить»):
- propose_product — предложить создать продукт (БЖУ на 100 г; можно указать fiber100 — клетчатку на 100 г и glycemic_index — гликемический индекс 0–100). Чтобы ИЗМЕНИТЬ уже существующий продукт (поправить БЖУ, название и т.п.), сначала вызови list_products, найди нужный продукт и передай его product_id — тогда продукт будет обновлён, а не создан заново. Без product_id создаётся новый продукт;
- propose_recipe — предложить создать рецепт. Ингредиенты ссылаются на product_id из list_products. ОБЯЗАТЕЛЬНО сначала вызови list_products, чтобы взять реальные id. Если нужного продукта нет, не выдумывай id: вместо этого попроси пользователя сначала создать продукт (или предложи его через propose_product);
- propose_copy_day — предложить скопировать блюда одного дня плана на другие дни (целевые дни будут заменены копией исходного). ОБЯЗАТЕЛЬНО сначала вызови list_meal_plans, чтобы взять реальные plan_id и даты (формат YYYY-MM-DD) — не выдумывай их. Для «всей недели» перечисли в target_dates все остальные дни плана.
- propose_add_to_plan — предложить добавить продукт (по граммам) прямо в ячейку плана (день × приём пищи). ОБЯЗАТЕЛЬНО сначала вызови list_meal_plans (чтобы взять реальный plan_id и дату) и list_products (чтобы взять реальный product_id) — не выдумывай их. Если нужного продукта нет, сначала предложи создать его через propose_product.

Когда пользователь просит добавить/создать/сохранить продукт или рецепт — ВСЕГДА вызывай соответствующий инструмент (propose_product/propose_recipe) с конкретными значениями, а не описывай их текстом. БЖУ и калорийность оцени сам по названию продукта (пользователь сможет поправить перед применением). У продукта обязательно должно быть название: если из запроса непонятно, какой именно продукт нужен, задай один короткий уточняющий вопрос, и как только название известно — сразу вызывай propose_product.

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
		Name:        toolListMealPlans,
		Description: "Вернуть планы питания пользователя с id, датами и блюдами по дням.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		Name:        toolProposeProduct,
		Description: "Предложить пользователю создать или обновить продукт. Не применяет изменение — пользователь подтверждает. Передай product_id (из list_products), чтобы обновить существующий продукт вместо создания нового.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"product_id":     map[string]any{"type": "integer", "description": "id существующего продукта для обновления; не указывай, чтобы создать новый"},
				"name":           map[string]any{"type": "string"},
				"category":       map[string]any{"type": "string"},
				"brand":          map[string]any{"type": "string"},
				"kcal100":        map[string]any{"type": "number"},
				"protein100":     map[string]any{"type": "number"},
				"fat100":         map[string]any{"type": "number"},
				"carbs100":       map[string]any{"type": "number"},
				"fiber100":       map[string]any{"type": "number"},
				"glycemic_index": map[string]any{"type": "number"},
				"source":         map[string]any{"type": "string"},
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
	{
		Name:        toolProposeCopyDay,
		Description: "Предложить скопировать блюда одного дня плана на другие дни (целевые дни заменяются копией). Даты берутся из list_meal_plans.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan_id":     map[string]any{"type": "integer"},
				"source_date": map[string]any{"type": "string", "description": "Исходный день, YYYY-MM-DD"},
				"target_dates": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string", "description": "YYYY-MM-DD"},
				},
			},
			"required": []string{"plan_id", "source_date", "target_dates"},
		},
	},
	{
		Name:        toolProposeAddToPlan,
		Description: "Предложить добавить продукт (по граммам) в ячейку плана (день × приём пищи). plan_id и день берутся из list_meal_plans, product_id — из list_products.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan_id":    map[string]any{"type": "integer"},
				"day_date":   map[string]any{"type": "string", "description": "День, YYYY-MM-DD"},
				"meal_slot":  map[string]any{"type": "string", "description": "Приём пищи, напр. breakfast/lunch/dinner"},
				"product_id": map[string]any{"type": "integer"},
				"grams":      map[string]any{"type": "number"},
			},
			"required": []string{"plan_id", "day_date", "meal_slot", "product_id", "grams"},
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

	switch toolName {
	case toolProposeCopyDay:
		if _, ok := payload["plan_id"]; !ok {
			return Proposal{}, errors.New("copy proposal is missing plan_id")
		}
		if src, _ := payload["source_date"].(string); src == "" {
			return Proposal{}, errors.New("copy proposal is missing source_date")
		}
		if targets, ok := payload["target_dates"].([]any); !ok || len(targets) == 0 {
			return Proposal{}, errors.New("copy proposal is missing target_dates")
		}
		return Proposal{Type: "copy_day", Payload: payload}, nil
	case toolProposeAddToPlan:
		if _, ok := payload["plan_id"]; !ok {
			return Proposal{}, errors.New("add_to_plan proposal is missing plan_id")
		}
		if d, _ := payload["day_date"].(string); d == "" {
			return Proposal{}, errors.New("add_to_plan proposal is missing day_date")
		}
		if slot, _ := payload["meal_slot"].(string); slot == "" {
			return Proposal{}, errors.New("add_to_plan proposal is missing meal_slot")
		}
		if _, ok := payload["product_id"]; !ok {
			return Proposal{}, errors.New("add_to_plan proposal is missing product_id")
		}
		if _, ok := payload["grams"]; !ok {
			return Proposal{}, errors.New("add_to_plan proposal is missing grams")
		}
		return Proposal{Type: "add_to_plan", Payload: payload}, nil
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
