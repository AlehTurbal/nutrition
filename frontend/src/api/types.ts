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

export interface ShoppingList {
  items: ShoppingItem[];
  totals: Macros;
  by_slot: SlotMacros[];
}
