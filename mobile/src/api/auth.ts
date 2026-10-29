import { getJSON, sendJSON } from "./client";

// Mirrors auth.User (server/internal/auth/service.go).
export type User = { username: string };

type LoginResponse = { token: string; user: User };

// login exchanges a username and password for a session token. A wrong pair is an ApiError
// with status 401; too many attempts, 429.
export function login(username: string, password: string): Promise<LoginResponse> {
  return sendJSON<LoginResponse>("POST", "/api/auth/login", { username, password });
}

// logout ends this device's session on the server.
export function logout(): Promise<void> {
  return sendJSON<void>("POST", "/api/auth/logout");
}

// me is who the current token belongs to: the launch-time check that it still works.
export function me({ signal }: { signal?: AbortSignal } = {}): Promise<User> {
  return getJSON<User>("/api/auth/me", { signal });
}
