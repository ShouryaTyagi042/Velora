import { router } from "expo-router";
import { useCallback } from "react";
import { StyleSheet, View } from "react-native";

import { listMedia, listSeries, MediaKind, Section, type SeriesCount } from "@/api/media";
import { Button } from "@/components/Button";
import { CoverCard } from "@/components/CoverCard";
import { GroupList } from "@/components/GroupList";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { useRefetchOnLibraryChange } from "@/hooks/useLibraryChanges";
import { useRescan } from "@/hooks/useRescan";
import { plural } from "@/lib/format";
import { space } from "@/theme";

// Anime: one banner per series (anime/<series>/ on the server), its first episode as the picture,
// one per row. A series opens its episodes. Episodes outside any series folder (films, OVAs) are
// under All anime.
const fetchSeries = async (signal: AbortSignal) => {
  const [series, any] = await Promise.all([
    listSeries({ signal }),
    listMedia({ kind: MediaKind.Video, section: Section.Anime, limit: 1, signal }),
  ]);
  return { series, anyAnime: any.items.length > 0 };
};

const renderSeries = (s: SeriesCount) => (
  <CoverCard
    name={s.name}
    count={plural(s.episodeCount, "episode")}
    cover={s.cover}
    icon="tv"
    onPress={() => router.push({ pathname: "/series/[name]", params: { name: s.name } })}
  />
);

export default function Anime() {
  const { state, refreshing, refetch } = useAsync(fetchSeries);
  const { scanning, rescan } = useRescan(refetch);
  useRefetchOnLibraryChange(refetch);
  const series = state.status === "ok" ? { status: "ok" as const, data: state.data.series } : state;
  const count = state.status === "ok" ? state.data.series.length : 0;
  const keyOf = useCallback((s: SeriesCount) => s.name, []);

  const header =
    state.status === "ok" && state.data.anyAnime ? (
      <View style={styles.header}>
        <Text muted style={styles.lede}>
          {count === 0 ? "No series folders yet." : `${plural(count, "series", "series")} on the shelf.`}
        </Text>
        <Button
          label="All anime"
          variant="secondary"
          icon={{ sf: "square.grid.2x2", md: "grid_view" }}
          onPress={() => router.push("/episodes")}
          style={styles.all}
        />
      </View>
    ) : undefined;

  return (
    <GroupList
      state={series}
      keyOf={keyOf}
      render={renderSeries}
      aspectRatio={16 / 8}
      refreshing={scanning || refreshing}
      onRefresh={rescan}
      onRetry={refetch}
      header={header}
      empty={{
        title: "No anime yet",
        detail: "Put episodes in anime/ on the server, one folder per series (anime/Trigun/Episode 1.mkv), then pull down to rescan.",
      }}
    />
  );
}

const styles = StyleSheet.create({
  header: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: space.md, marginBottom: space.lg },
  lede: { flex: 1 },
  all: { paddingHorizontal: space.md },
});
