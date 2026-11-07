// formatBytes shows a file size the way people read it: 289 MB, not 303057905.
export function formatBytes(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value < 10 && unit > 0 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

// formatDuration shows a playback position: 0:07, 4:32, 1:02:09. NaN (duration unknown) shows 0:00.
export function formatDuration(seconds: number): string {
  const total = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = String(total % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`;
}

// formatResolution names a video's size the way people do: 1080p, 4K, or 853×480.
export function formatResolution(width: number, height: number): string {
  if (width >= 3840) return "4K";
  if ([2160, 1440, 1080, 720, 480].includes(height)) return `${height}p`;
  return `${width}×${height}`;
}

// plural is a count with its noun: "1 video", "12 videos", "3 people".
export function plural(n: number, one: string, many = `${one}s`): string {
  return `${n} ${n === 1 ? one : many}`;
}

// formatAgo says how long ago an instant was, the way people say it: "just now", "5 minutes ago",
// "yesterday", "3 days ago", then a date.
export function formatAgo(iso: string, now = Date.now()): string {
  const s = Math.max(0, (now - Date.parse(iso)) / 1000);
  if (s < 60) return "just now";
  if (s < 3600) return `${plural(Math.floor(s / 60), "minute")} ago`;
  if (s < 86_400) return `${plural(Math.floor(s / 3600), "hour")} ago`;
  const days = Math.floor(s / 86_400);
  if (days === 1) return "yesterday";
  if (days < 30) return `${days} days ago`;
  return formatDate(iso);
}

// formatViews is a view count the way streaming apps show one: "1 view", "842 views", "12K views".
export function formatViews(n: number): string {
  if (n < 1000) return plural(n, "view");
  const k = n / 1000;
  return `${k < 10 ? k.toFixed(1).replace(/\.0$/, "") : Math.round(k)}K views`;
}
