import { useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { StyleSheet, useWindowDimensions, View } from "react-native";

import { listMedia, MediaKind, type Media } from "@/api/media";
import { MediaCard } from "@/components/MediaCard";
import { MediaGrid } from "@/components/MediaGrid";
import { Rail } from "@/components/Rail";
import { TypeFilter } from "@/components/TypeFilter";
import { VideoList } from "@/components/VideoList";
import { useAsync } from "@/hooks/useAsync";
import { useRefetchOnLibraryChange } from "@/hooks/useLibraryChanges";
import { usePages } from "@/hooks/usePages";
import { useRescan } from "@/hooks/useRescan";
import { parseMediaType, typeFilter, type MediaType } from "@/lib/mediaType";
import { openMedia } from "@/navigation/openMedia";
import { space } from "@/theme";

const EMPTY = {
  all: { title: "No favourites yet", detail: "Tap the heart on anything's details page and it lands here." },
  videos: { title: "No favourite videos", detail: "Tap the heart on a video's details page and it lands here." },
  anime: { title: "No favourite anime", detail: "Tap the heart on an episode's details page and it lands here." },
  comics: { title: "No favourite comics", detail: "Open a comic, tap ⓘ, then the heart, and it lands here." },
} as const;

// Favourites: what you've hearted, cut by type (?type=videos|anime|comics). Videos and anime are
// one per row, like every list of videos; comics are covers, three across; All shows your comics
// as a row of covers over your videos.
export default function Favourites() {
  const type = parseMediaType(useLocalSearchParams<{ type?: string }>().type);
  return type === "comics" ? <FavouriteComics /> : <FavouriteVideos type={type} />;
}

const fetchComicRail = async (signal: AbortSignal) =>
  (await listMedia({ kind: MediaKind.Comic, favourites: true, limit: 50, signal })).items;
const cover = (m: Media) => <MediaCard media={m} onPress={openMedia} style={styles.cover} />;

function FavouriteVideos({ type }: { type: Exclude<MediaType, "comics"> }) {
  const all = type === "all";
  const comics = useAsync(fetchComicRail);
  useRefetchOnLibraryChange(comics.refetch);
  const covers = all && comics.state.status === "ok" ? comics.state.data : [];
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) =>
      listMedia({ kind: MediaKind.Video, ...typeFilter(type), favourites: true, sort: "recent", after, signal }),
    [type],
  );
  return (
    <VideoList
      fetchPage={fetchPage}
      header={
        <View style={styles.header}>
          <TypeFilter value={type} />
          {covers.length > 0 && (
            <View style={styles.rail}>
              <Rail title="Comics" items={covers} keyOf={(m) => m.id} render={cover} seeAll={{ pathname: "/favourites", params: { type: "comics" } }} />
            </View>
          )}
        </View>
      }
      empty={all && covers.length > 0 ? { title: "No favourite videos yet", detail: EMPTY.videos.detail } : EMPTY[type]}
    />
  );
}

function FavouriteComics() {
  const { width } = useWindowDimensions();
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => listMedia({ kind: MediaKind.Comic, favourites: true, sort: "recent", after, signal }),
    [],
  );
  const { state, refreshing, refetch, loadMore, loadingMore, moreError } = usePages(fetchPage);
  useRefetchOnLibraryChange(refetch);
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
      header={
        <View style={styles.gridHeader}>
          <TypeFilter value="comics" inset={space.sm} />
        </View>
      }
      empty={EMPTY.comics}
    />
  );
}

const styles = StyleSheet.create({
  header: { gap: space.lg, marginBottom: space.lg },
  // The rail draws its own side padding; the list's would double it.
  rail: { marginHorizontal: -space.lg },
  cover: { width: 112 },
  gridHeader: { paddingBottom: space.md },
});
