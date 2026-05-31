import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./client";
import type {
  Profile,
  ProfileInput,
  TargetsResponse,
  WeightEntry,
} from "./types";

export function useProfile() {
  return useQuery({
    queryKey: ["profile"],
    queryFn: async () => {
      try {
        return await api.get<Profile>("/api/profile");
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
  });
}

export function useSaveProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ProfileInput) => api.put<Profile>("/api/profile", input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["profile"] });
      qc.invalidateQueries({ queryKey: ["targets"] });
    },
  });
}

export function useWeights() {
  return useQuery({
    queryKey: ["weights"],
    queryFn: () => api.get<WeightEntry[]>("/api/weights"),
  });
}

export function useAddWeight() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (weight_kg: number) =>
      api.post<WeightEntry>("/api/weights", { weight_kg }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["weights"] });
      qc.invalidateQueries({ queryKey: ["targets"] });
    },
  });
}

export function useTargets() {
  return useQuery({
    queryKey: ["targets"],
    queryFn: async () => {
      try {
        return await api.get<TargetsResponse>("/api/targets");
      } catch (e) {
        if (e instanceof ApiError && e.status === 400) return null;
        throw e;
      }
    },
  });
}
