import { useMutation } from "@tanstack/react-query";
import { api } from "./client";
import { setToken } from "../lib/auth";

interface TokenResponse {
  token: string;
}

function useAuth(path: string) {
  return useMutation({
    mutationFn: (creds: { email: string; password: string }) =>
      api.post<TokenResponse>(path, creds),
    onSuccess: (data) => setToken(data.token),
  });
}

export const useLogin = () => useAuth("/api/auth/login");
export const useRegister = () => useAuth("/api/auth/register");
