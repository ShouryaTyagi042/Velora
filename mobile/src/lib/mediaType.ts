import { MediaKind, Section } from "@/api/media";

// MediaType is how the personal lists (Favourites, Bin, Tags) are cut: everything, or one shelf.
// Videos are films (movies/), Anime is anime/, and both are videos to the server.
export type MediaType = "all" | "videos" | "anime" | "comics";

export const MEDIA_TYPES: { type: MediaType; label: string }[] = [
  { type: "all", label: "All" },
  { type: "videos", label: "Videos" },
  { type: "anime", label: "Anime" },
  { type: "comics", label: "Comics" },
];

// Tags are on videos only, so their lists offer no Comics.
export const VIDEO_TYPES = MEDIA_TYPES.filter((t) => t.type !== "comics");

// parseMediaType reads ?type= back; anything else is All.
export function parseMediaType(value: string | undefined, allowed = MEDIA_TYPES): MediaType {
  return allowed.find((t) => t.type === value)?.type ?? "all";
}

// typeFilter is the list filter for a type.
export function typeFilter(t: MediaType): { kind?: MediaKind; section?: Section } {
  switch (t) {
    case "videos":
      return { kind: MediaKind.Video, section: Section.Movies };
    case "anime":
      return { kind: MediaKind.Video, section: Section.Anime };
    case "comics":
      return { kind: MediaKind.Comic };
  }
  return {};
}

// typeNoun is a type in a sentence: "comics", "anime", "videos", or "items" for All.
export function typeNoun(t: MediaType): string {
  return t === "all" ? "items" : t;
}
