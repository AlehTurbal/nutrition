import type { ActivityLevel, Goal, MealSlot, Sex } from "../api/types";

export const sexes: { value: Sex; label: string }[] = [
  { value: "male", label: "Мужской" },
  { value: "female", label: "Женский" },
];

export const activities: { value: ActivityLevel; label: string }[] = [
  { value: "sedentary", label: "Сидячий" },
  { value: "light", label: "Лёгкая активность" },
  { value: "moderate", label: "Умеренная" },
  { value: "active", label: "Высокая" },
  { value: "very_active", label: "Очень высокая" },
];

export const goals: { value: Goal; label: string }[] = [
  { value: "lose", label: "Похудение" },
  { value: "maintain", label: "Поддержание" },
  { value: "gain", label: "Набор" },
];

export const mealSlots: { value: MealSlot; label: string }[] = [
  { value: "breakfast", label: "Завтрак" },
  { value: "lunch", label: "Обед" },
  { value: "dinner", label: "Ужин" },
  { value: "snack", label: "Перекус" },
];

export function slotLabel(slot: string): string {
  return mealSlots.find((s) => s.value === slot)?.label ?? slot;
}

export function fmt(n: number): string {
  return n.toLocaleString("ru-RU", { maximumFractionDigits: 1 });
}
