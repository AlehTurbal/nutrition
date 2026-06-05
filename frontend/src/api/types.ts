// Wire types mirroring the Go backend JSON.

export type Sex = "male" | "female";
export type ActivityLevel =
  | "sedentary"
  | "light"
  | "moderate"
  | "active"
  | "very_active";
export type Goal = "lose" | "maintain" | "gain";
export type MealSlot = "breakfast" | "lunch" | "dinner" | "snack";

export interface Profile {
  user_id: number;
  sex: Sex;
  height_cm: number;
  age: number;
  activity_level: ActivityLevel;
  goal: Goal;
  protein_per_kg: number;
  fat_pct: number;
  meal_slots: MealSlot[];
  updated_at: string;
}

export type ProfileInput = Omit<Profile, "user_id" | "updated_at">;

export interface WeightEntry {
  id: number;
  user_id: number;
  weight_kg: number;
  recorded_at: string;
}

export interface Targets {
  bmr: number;
  tdee: number;
  calories: number;
  protein_g: number;
  fat_g: number;
  carbs_g: number;
}

export interface MealTarget {
  slot: string;
  calories: number;
  protein_g: number;
  fat_g: number;
  carbs_g: number;
}

export interface TargetsResponse {
  weight_kg: number;
  targets: Targets;
  meal_slots: string[];
  per_meal: MealTarget[];
}

export interface Product {
  id: number;
  user_id: number;
  name: string;
  category: string;
  brand: string;
  kcal100: number | null;
  protein100: number | null;
  fat100: number | null;
  carbs100: number | null;
  fiber100: number | null;
  glycemic_index: number | null;
  source: string;
  created_at: string;
  updated_at: string;
}

export interface ProductInput {
  name: string;
  category?: string;
  brand?: string;
  kcal100: number | null;
  protein100: number | null;
  fat100: number | null;
  carbs100: number | null;
  fiber100: number | null;
  glycemic_index: number | null;
}

export interface Macros {
  kcal: number;
  protein: number;
  fat: number;
  carbs: number;
  complete: boolean;
}

export interface Ingredient {
  id: number;
  product_id: number;
  product_name: string;
  grams: number;
  kcal100: number | null;
  protein100: number | null;
  fat100: number | null;
  carbs100: number | null;
}

export interface Recipe {
  id: number;
  user_id: number;
  name: string;
  servings: number;
  instructions: string;
  meal_types: MealSlot[];
  source: string;
  created_at: string;
  ingredients?: Ingredient[];
}

export interface RecipeResponse {
  recipe: Recipe;
  macros: Macros;
  macros_per_serving: Macros;
}

export interface RecipeInput {
  name: string;
  servings: number;
  instructions?: string;
  meal_types: MealSlot[];
  ingredients: { product_id: number; grams: number }[];
}

export interface PlanItem {
  id: number;
  meal_plan_id: number;
  day_date: string;
  meal_slot: string;
  recipe_id: number;
  recipe_name: string;
  servings: number;
  // Set instead of the recipe fields when the cell holds a raw product.
  product_id?: number;
  product_name?: string;
  grams?: number;
}

export interface Plan {
  id: number;
  user_id: number;
  name: string;
  start_date: string;
  end_date: string;
  created_at: string;
  items: PlanItem[];
}

export interface ShoppingItem {
  product_id: number;
  product_name: string;
  grams: number;
  macros: Macros;
}

export interface SlotMacros {
  slot: string;
  macros: Macros;
}

export interface DayMacros {
  date: string;
  macros: Macros;
}

export interface ShoppingList {
  items: ShoppingItem[];
  totals: Macros;
  by_slot: SlotMacros[];
  by_day: DayMacros[];
}

export interface StoreMatchItem {
  needed: string;
  matched: string;
  found: boolean;
}

export interface SavedStoreMatch {
  id: number;
  user_id: number;
  meal_plan_id: number;
  created_at: string;
  items: StoreMatchItem[];
}

export interface GeneratedIngredient {
  name: string;
  grams: number;
}

export interface GeneratedRecipe {
  name: string;
  servings: number;
  instructions: string;
  meal_types: MealSlot[];
  ingredients: GeneratedIngredient[];
}

export interface MatchedIngredient {
  name: string;
  grams: number;
  product_id: number | null;
}

export interface GenerateResponse {
  recipe: GeneratedRecipe;
  ingredients: MatchedIngredient[];
}

// --- chat assistant ---

export interface ChatThread {
  id: number;
  user_id: number;
  title: string;
  created_at: string;
}

export interface ChatMessage {
  id: number;
  thread_id: number;
  role: "user" | "assistant";
  content: string;
  created_at: string;
}

export interface CopyDayPayload {
  plan_id: number;
  source_date: string;
  target_dates: string[];
}

export interface AddToPlanPayload {
  plan_id: number;
  day_date: string;
  meal_slot: string;
  product_id: number;
  grams: number;
}

// A previewed mutation the user confirms. payload is shaped exactly like the
// body of the matching endpoint (POST /api/products, /api/recipes,
// /api/meal-plans/{id}/copy-day, or /api/meal-plans/{id}/items).
export type ChatProposal =
  | { type: "product"; payload: ProductInput & { product_id?: number } }
  | { type: "recipe"; payload: RecipeInput }
  | { type: "copy_day"; payload: CopyDayPayload }
  | { type: "add_to_plan"; payload: AddToPlanPayload };

export interface ChatSendResponse {
  message: ChatMessage;
  proposals: ChatProposal[];
}
