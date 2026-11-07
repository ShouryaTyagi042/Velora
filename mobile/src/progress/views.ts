import { useCallback, useRef } from "react";

import { recordView } from "@/api/media";
import { libraryChanged } from "@/hooks/useLibraryChanges";

// WATCHED_FOR is how much of a video has to be played, in seconds, before it counts as a view:
// enough to mean it, not so much that skimming a long film never counts. A shorter video counts
// at its end.
export const WATCHED_FOR = 30;

// useViewOnce returns a function that records one view of id, the first time it's called while
// the screen is open; later calls do nothing. A failed send is dropped: a missed view isn't worth
// a retry queue (progress has one; views are a statistic).
export function useViewOnce(id: string): () => void {
  const sent = useRef(false);
  return useCallback(() => {
    if (sent.current) return;
    sent.current = true;
    recordView(id).then(
      (r) => {
        if (r.counted) libraryChanged(); // lists showing the count refresh when shown again
      },
      () => {},
    );
  }, [id]);
}
