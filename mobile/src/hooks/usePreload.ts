import { Image } from "expo-image";
import { useEffect } from "react";

import { authHeaders } from "@/api/client";

// usePreload fetches the next two pages into expo-image's disk cache while page n is on screen.
// Forward only: reading forward is the overwhelming case, and going back is a cache hit anyway.
export function usePreload(urls: string[], n: number) {
  useEffect(() => {
    const next = urls.slice(n + 1, n + 3);
    if (next.length > 0) void Image.prefetch(next, { cachePolicy: "disk", headers: authHeaders() });
  }, [urls, n]);
}
