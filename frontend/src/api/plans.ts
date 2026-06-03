import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type { CopyDayPayload, Plan, PlanItem, ShoppingList } from "./types";

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

export function useAddItem(planId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      day_date: string;
      meal_slot: string;
      recipe_id: number;
      servings: number;
    }) => api.post<PlanItem>(`/api/meal-plans/${planId}/items`, input),
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
