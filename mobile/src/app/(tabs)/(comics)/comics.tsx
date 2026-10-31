import { useWindowDimensions } from "react-native";

import { listMedia, MediaKind } from "@/api/media";
import { MediaGrid } from "@/components/MediaGrid";
import { usePages } from "@/hooks/usePages";
import { useRescan } from "@/hooks/useRescan";

const fetchComics = (after: string | undefined, signal: AbortSignal) => listMedia({ kind: MediaKind.Comic, after, signal });

// Comics: every comic as a cover, three across like a shelf (four on a tablet). Covers are 2:3,
// so more fit in a row than video posters do.
export default function Comics() {
  const { width } = useWindowDimensions();
  const { state, refreshing, refetch, loadMore, loadingMore, moreError } = usePages(fetchComics);
  const { scanning, rescan } = useRescan(refetch);

  return (
    <MediaGrid
      state={state}
      columns={width >= 600 ? 4 : 3}
      aspectRatio={2 / 3}
      refreshing={scanning || refreshing}
      onRefresh={rescan}
      onRetry={refetch}
      onEndReached={loadMore}
      loadingMore={loadingMore}
      moreError={moreError}
      empty={{
        title: "No comics yet",
        detail: "Put each comic's page images in its own folder inside comics/ (comics/Watchmen/001.jpg, …) and pull down to rescan.",
      }}
    />
  );
}
