import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ScrollView, StyleSheet, useWindowDimensions } from "react-native";

import { listActors, listMedia, listSeries, MediaKind, Section } from "@/api/media";
import { Chip } from "@/components/Chip";
import { MediaGrid } from "@/components/MediaGrid";
import { useAsync } from "@/hooks/useAsync";
import { usePages } from "@/hooks/usePages";
import { useRescan } from "@/hooks/useRescan";
import { space } from "@/theme";

// A chip in the filter bar: a name, and how many videos it filters down to.
type FilterChip = { name: string; count: number };

// What differs between the Videos and Anime tabs: the title, and what the chips filter by. Videos filter by actor (movies/<actor>/); Anime by series (anime/<series>/), which is a show,
// not a person: the server keeps the two apart, so neither list fills up with the other.
const shelves = {
  [Section.Movies]: {
    title: "Videos",
    chipParam: "actor",
    unit: ["video", "videos"],
    fetchChips: async (signal: AbortSignal): Promise<FilterChip[]> =>
      (await listActors({ section: Section.Movies, signal })).map((a) => ({ name: a.name, count: a.videoCount })),
  },
  [Section.Anime]: {
    title: "Anime",
    chipParam: "series",
    unit: ["episode", "episodes"],
    fetchChips: async (signal: AbortSignal): Promise<FilterChip[]> =>
      (await listSeries({ signal })).map((s) => ({ name: s.name, count: s.episodeCount })),
  },
} as const;

type Props = { section: Section };

// VideoShelf is one video tab: every video in a section, filtered by actor (Videos) or series
// (Anime). The filter lives in the route (/videos?actor=Keanu+Reeves, /anime?series=Trigun), so
// back restores the previous one, a chip on Details links straight here, and the URL is a deep
// link. ?actor= works on Anime too: a voice actor added by hand links here from Details.
export function VideoShelf({ section }: Props) {
  const shelf = shelves[section];
  const params = useLocalSearchParams<{ actor?: string; series?: string }>();
  const actor = params.actor || undefined;
  const series = (section === Section.Anime && params.series) || undefined;
  const selected = shelf.chipParam === "series" ? series : actor;
  const { width } = useWindowDimensions();

  // New functions only when a filter (or the section) changes, and that is what starts the list again.
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) =>
      listMedia({ kind: MediaKind.Video, section, series, actor, after, signal }),
    [section, series, actor],
  );
  const { state, refreshing, refetch, loadMore, loadingMore, moreError } = usePages(fetchPage);
  const chips = useAsync(shelf.fetchChips);
  const refetchBoth = () => {
    refetch();
    chips.refetch();
  };
  const { scanning, rescan } = useRescan(refetchBoth);

  // The chips scroll with the grid, under the glass header. With nothing to filter by, no bar.
  // "All" clears both filters: an actor link from Details can land here with ?actor= set.
  const names = chips.state.status === "ok" ? chips.state.data : [];
  const filters =
    names.length === 0 ? undefined : (
      <ScrollView horizontal contentContainerStyle={styles.filters} showsHorizontalScrollIndicator={false}>
        <Chip
          label="All"
          selected={!actor && !series}
          onPress={() => router.setParams({ actor: undefined, series: undefined })}
        />
        {names.map((c) => (
          <Chip
            key={c.name}
            label={c.name}
            accessibilityLabel={`${c.name}, ${c.count} ${shelf.unit[c.count === 1 ? 0 : 1]}`}
            selected={selected === c.name}
            onPress={() => router.setParams({ [shelf.chipParam]: selected === c.name ? undefined : c.name })}
          />
        ))}
      </ScrollView>
    );

  const filter = series ?? actor;
  return (
    <>
      <Stack.Screen options={{ title: filter ?? shelf.title }} />
      <MediaGrid
        state={state}
        columns={width >= 600 ? 3 : 2}
        aspectRatio={16 / 9}
        refreshing={scanning || refreshing}
        onRefresh={rescan}
        onRetry={refetch}
        onEndReached={loadMore}
        loadingMore={loadingMore}
        moreError={moreError}
        header={filters}
        empty={
          filter
            ? {
                title: `Nothing from ${filter}`,
                detail: "The files may have been moved. Pull down to rescan, or pick All.",
              }
            : {
                title: `No ${shelf.title.toLowerCase()} yet`,
                detail:
                  section === Section.Anime
                    ? "Put episodes in anime/, one folder per series (anime/Trigun/Episode 1.mkv), and pull down to rescan."
                    : "Put video files in movies/ (in a folder named after the actor, if you like) and pull down to rescan.",
              }
        }
      />
    </>
  );
}

const styles = StyleSheet.create({
  filters: { paddingHorizontal: space.sm, paddingBottom: space.md, gap: space.sm },
});
