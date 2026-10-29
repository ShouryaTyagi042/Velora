import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ScrollView, StyleSheet, useWindowDimensions } from "react-native";

import { listActors, listMedia, MediaKind } from "@/api/media";
import { Chip } from "@/components/Chip";
import { MediaGrid } from "@/components/MediaGrid";
import { useAsync } from "@/hooks/useAsync";
import { useRescan } from "@/hooks/useRescan";
import { space } from "@/theme";

const fetchActors = (signal: AbortSignal) => listActors({ signal });

// Videos: every video, filtered by actor. The filter lives in the route (/videos?actor=Keanu+Reeves),
// so back restores the previous one, an actor chip on Details links straight here, and the URL is
// a deep link.
export default function Videos() {
  const params = useLocalSearchParams<{ actor?: string }>();
  const actor = params.actor || undefined;
  const { width } = useWindowDimensions();

  // A new function only when actor changes, and that is what starts a new request.
  const fetcher = useCallback(
    (signal: AbortSignal) => listMedia({ kind: MediaKind.Video, actor, signal }),
    [actor],
  );
  const { state, refreshing, refetch } = useAsync(fetcher);
  const actors = useAsync(fetchActors);
  const refetchBoth = () => {
    refetch();
    actors.refetch();
  };
  const { scanning, rescan } = useRescan(refetchBoth);

  // The actors scroll with the grid, under the glass header. Without actors there's nothing to
  // filter by, so no bar at all.
  const names = actors.state.status === "ok" ? actors.state.data : [];
  const filters =
    names.length === 0 ? undefined : (
      <ScrollView horizontal contentContainerStyle={styles.filters} showsHorizontalScrollIndicator={false}>
        <Chip label="All" selected={!actor} onPress={() => router.setParams({ actor: undefined })} />
        {names.map((a) => (
          <Chip
            key={a.name}
            label={a.name}
            accessibilityLabel={`${a.name}, ${a.videoCount} ${a.videoCount === 1 ? "video" : "videos"}`}
            selected={actor === a.name}
            onPress={() => router.setParams({ actor: actor === a.name ? undefined : a.name })}
          />
        ))}
      </ScrollView>
    );

  return (
    <>
      <Stack.Screen options={{ title: actor ?? "Videos" }} />
      <MediaGrid
        state={state}
        columns={width >= 600 ? 3 : 2}
        aspectRatio={16 / 9}
        refreshing={scanning || refreshing}
        onRefresh={rescan}
        onRetry={refetch}
        header={filters}
        empty={
          actor
            ? { title: `No videos with ${actor}`, detail: "Their videos may have been moved. Pull down to rescan, or pick All." }
            : { title: "No videos yet", detail: "Put video files in movies/ (in a folder named after the actor, if you like) and pull down to rescan." }
        }
      />
    </>
  );
}

const styles = StyleSheet.create({
  filters: { paddingHorizontal: space.sm, paddingBottom: space.md, gap: space.sm },
});
