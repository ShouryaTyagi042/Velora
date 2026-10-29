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
