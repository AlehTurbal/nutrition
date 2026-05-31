import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./client";
import type { GenerateResponse, SavedStoreMatch } from "./types";

export function useStoreMatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { plan_id: number; store_text: string }) =>
      api.post<SavedStoreMatch>("/api/stores/match", input),
    onSuccess: (_d, vars) =>
      qc.invalidateQueries({ queryKey: ["store-match", vars.plan_id] }),
  });
}

export function useStoreMatches(planId: number | null) {
  return useQuery({
    queryKey: ["store-match", planId],
    enabled: planId != null,
    queryFn: async () => {
      try {
        return await api.get<SavedStoreMatch>(`/api/stores/matches/${planId}`);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
  });
}

export function useGenerateRecipe() {
  return useMutation({
    mutationFn: (input: {
      description: string;
      meal_types: string[];
      servings: number;
    }) => api.post<GenerateResponse>("/api/recipes/generate", input),
  });
}
