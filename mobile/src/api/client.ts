const baseUrl = process.env.EXPO_PUBLIC_API_URL;
if (!baseUrl) throw new Error("EXPO_PUBLIC_API_URL is not set");

export async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return (await res.json()) as T;
}
