import { useCallback } from "react";

import { listMedia, MediaKind, Section } from "@/api/media";
import { VideoList } from "@/components/VideoList";

// All videos: every film in movies/, A to Z, whether or not anyone is named in it.
export default function AllVideos() {
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => listMedia({ kind: MediaKind.Video, section: Section.Movies, after, signal }),
    [],
  );
  return (
    <VideoList
      fetchPage={fetchPage}
      empty={{ title: "No videos yet", detail: "Put video files in movies/ on the server and pull down to rescan." }}
    />
  );
}
