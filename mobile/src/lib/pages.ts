// LATE_FROM is where a comic's previews start, as a fraction of its pages: past the cover, the
// credits and the setup, into the part of the story that shows what the comic is.
export const LATE_FROM = 0.6;

// latePages is up to n page numbers (0-based) spread evenly across the last 40% of a comic of
// count pages, in reading order. A comic too short for that gives what it has.
export function latePages(count: number, n: number): number[] {
  if (count <= 0 || n <= 0) return [];
  const first = Math.min(count - 1, Math.floor(count * LATE_FROM));
  const span = count - first; // pages from first to the end
  if (span <= n) return Array.from({ length: span }, (_, i) => first + i);
  // n evenly spaced picks over the span, the first at `first`, the last at the final page.
  return Array.from({ length: n }, (_, i) => first + Math.round((i * (span - 1)) / (n - 1 || 1)));
}
