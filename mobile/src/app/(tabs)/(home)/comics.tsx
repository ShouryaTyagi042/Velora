import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { useWindowDimensions } from "react-native";

import { listMedia, MediaKind } from "@/api/media";
import { MediaGrid } from "@/components/MediaGrid";
import { SortButton } from "@/components/SortButton";
import { usePages } from "@/hooks/usePages";
import { useRefetchOnLibraryChange } from "@/hooks/useLibraryChanges";
import { useRescan } from "@/hooks/useRescan";

// Comics: every comic as a cover, three across like a shelf (four on a tablet). Covers are 2:3,
// so more fit in a row than video posters do. A to Z by default; the header button flips it.
// The order lives in the route (/comics?order=desc) like the Videos actor filter, so it survives
// leaving and coming back. The server sorts (the list is paged), so a flip starts the list again.
export default function Comics() {
  const params = useLocalSearchParams<{ order?: string }>();
  const descending = params.order === "desc";
  const { width } = useWindowDimensions();

  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) =>
      listMedia({ kind: MediaKind.Comic, sort: descending ? "title_desc" : "title", after, signal }),
    [descending],
  );
  const { state, refreshing, refetch, loadMore, loadingMore, moreError } = usePages(fetchPage);
  const { scanning, rescan } = useRescan(refetch);
  useRefetchOnLibraryChange(refetch);

  return (
    <>
      <Stack.Screen
        options={{
          headerRight: () => (
            <SortButton descending={descending} onToggle={() => router.setParams({ order: descending ? undefined : "desc" })} />
          ),
        }}
      />
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
    </>
  );
}
