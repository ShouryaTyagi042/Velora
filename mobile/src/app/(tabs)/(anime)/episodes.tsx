import { useCallback } from "react";

import { listMedia, MediaKind, Section } from "@/api/media";
import { VideoList } from "@/components/VideoList";

// All anime: every episode and film under anime/, A to Z, series or not.
export default function AllAnime() {
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => listMedia({ kind: MediaKind.Video, section: Section.Anime, after, signal }),
    [],
  );
  return (
    <VideoList
      fetchPage={fetchPage}
      empty={{ title: "No anime yet", detail: "Put episodes in anime/ on the server and pull down to rescan." }}
    />
  );
}
