export const baseUrl = process.env.EXPO_PUBLIC_API_URL;
if (!baseUrl) throw new Error("EXPO_PUBLIC_API_URL is not set");

// The server's error envelope (server/internal/api/respond.go).
type ErrorEnvelope = { error: { code: string; message: string } };

// ApiError is a failed response from the server: it answered, but not with 2xx.
// A network failure (server down, Wi-Fi off) is a plain TypeError from fetch instead.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function getJSON<T>(path: string, { signal }: { signal?: AbortSignal } = {}): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, { signal });
  if (!res.ok) throw await toApiError(res);
  // The one place the untyped wire becomes a type. Nothing else in the app sees `any`.
  return (await res.json()) as T;
}

// sendJSON sends a write: body (if any) as JSON, and the response parsed as T. A 204 has no
// body, so it resolves to undefined; use T = void for those endpoints.
export async function sendJSON<T>(method: "POST" | "PUT" | "DELETE", path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw await toApiError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as ErrorEnvelope;
    return new ApiError(res.status, body.error.code, body.error.message);
  } catch {
    // Not our envelope (a proxy's HTML page, an empty body): keep the status at least.
    return new ApiError(res.status, "unknown", `${res.status} ${res.statusText}`);
  }
}

// isAbort tells a cancelled request (screen left, filter changed) from a real failure.
export function isAbort(err: unknown): boolean {
  return err instanceof Error && err.name === "AbortError";
}
