import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type {
  AddToPlanPayload,
  CopyDayPayload,
  Plan,
  PlanItem,
  ShoppingList,
} from "./types";

export function usePlans() {
  return useQuery({
    queryKey: ["plans"],
    queryFn: () => api.get<Plan[]>("/api/meal-plans"),
  });
}

export function usePlan(id: number | null) {
  return useQuery({
    queryKey: ["plan", id],
    queryFn: () => api.get<Plan>(`/api/meal-plans/${id}`),
    enabled: id != null,
  });
}

export function useCreatePlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; start_date: string; end_date: string }) =>
      api.post<Plan>("/api/meal-plans", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["plans"] }),
  });
}

export function useDeletePlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.del<void>(`/api/meal-plans/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["plans"] }),
  });
}

// useAddItem adds a recipe (recipe_id + servings) or a raw product
// (product_id + grams) into a plan cell — same endpoint, two payload shapes.
export function useAddItem(planId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (
      input:
        | { day_date: string; meal_slot: string; recipe_id: number; servings: number }
        | { day_date: string; meal_slot: string; product_id: number; grams: number },
    ) => api.post<PlanItem>(`/api/meal-plans/${planId}/items`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plan", planId] });
      qc.invalidateQueries({ queryKey: ["shopping", planId] });
    },
  });
}

export function useCopyDay(planId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { source_date: string; target_dates: string[] }) =>
      api.post<PlanItem[]>(`/api/meal-plans/${planId}/copy-day`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plan", planId] });
      qc.invalidateQueries({ queryKey: ["shopping", planId] });
    },
  });
}

// useApplyCopyDay applies a chat copy_day proposal, where the plan id travels in
// the payload rather than being fixed by the calling screen.
export function useApplyCopyDay() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CopyDayPayload) =>
      api.post<PlanItem[]>(`/api/meal-plans/${payload.plan_id}/copy-day`, {
        source_date: payload.source_date,
        target_dates: payload.target_dates,
      }),
    onSuccess: (_data, payload) => {
      qc.invalidateQueries({ queryKey: ["plan", payload.plan_id] });
      qc.invalidateQueries({ queryKey: ["shopping", payload.plan_id] });
      qc.invalidateQueries({ queryKey: ["plans"] });
    },
  });
}

// useApplyAddToPlan applies a chat add_to_plan proposal, where the plan id
// travels in the payload rather than being fixed by the calling screen.
export function useApplyAddToPlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: AddToPlanPayload) =>
      api.post<PlanItem>(`/api/meal-plans/${payload.plan_id}/items`, {
        day_date: payload.day_date,
        meal_slot: payload.meal_slot,
        product_id: payload.product_id,
        grams: payload.grams,
      }),
    onSuccess: (_data, payload) => {
      qc.invalidateQueries({ queryKey: ["plan", payload.plan_id] });
      qc.invalidateQueries({ queryKey: ["shopping", payload.plan_id] });
    },
  });
}

// useUpdateItem changes the quantity of one plan item — servings for a recipe
// item or grams for a product item (exactly one is sent).
export function useUpdateItem(planId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { itemId: number; servings?: number; grams?: number }) =>
      api.patch<PlanItem>(`/api/meal-plans/${planId}/items/${input.itemId}`, {
        servings: input.servings,
        grams: input.grams,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plan", planId] });
      qc.invalidateQueries({ queryKey: ["shopping", planId] });
    },
  });
}

export function useDeleteItem(planId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (itemId: number) =>
      api.del<void>(`/api/meal-plans/${planId}/items/${itemId}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plan", planId] });
      qc.invalidateQueries({ queryKey: ["shopping", planId] });
    },
  });
}

export function useShoppingList(planId: number | null) {
  return useQuery({
    queryKey: ["shopping", planId],
    queryFn: () => api.get<ShoppingList>(`/api/meal-plans/${planId}/shopping-list`),
    enabled: planId != null,
  });
}
