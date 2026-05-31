// Minimal JWT storage + a tiny pub/sub so components re-render on login/logout.

const KEY = "nutrition_token";
const listeners = new Set<() => void>();

export function getToken(): string | null {
  return localStorage.getItem(KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(KEY, token);
  listeners.forEach((l) => l());
}

export function clearToken(): void {
  localStorage.removeItem(KEY);
  listeners.forEach((l) => l());
}

export function subscribe(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
