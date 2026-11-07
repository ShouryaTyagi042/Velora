import { router } from "expo-router";
import { useCallback } from "react";
import { StyleSheet, View } from "react-native";

import { listActors, listMedia, MediaKind, Section, type ActorCount } from "@/api/media";
import { Button } from "@/components/Button";
import { CoverCard } from "@/components/CoverCard";
import { GroupList } from "@/components/GroupList";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { useRescan } from "@/hooks/useRescan";
import { plural } from "@/lib/format";
import { space } from "@/theme";

// The library's films, by who's in them (movies/<actor>/ on the server, plus names added in the
// app). One banner per actor, their newest video as the picture: the screen answers "who's
// here?" at a glance. Videos without an actor are one tap away under All videos.
const fetchActors = async (signal: AbortSignal) => {
  const [actors, all] = await Promise.all([
    listActors({ section: Section.Movies, signal }),
    // Only to know whether there are videos at all, and to say so in the header.
    listMedia({ kind: MediaKind.Video, section: Section.Movies, limit: 1, signal }),
  ]);
  return { actors, anyVideos: all.items.length > 0 };
};

export default function Actors() {
  const { state, refreshing, refetch } = useAsync(fetchActors);
  const { scanning, rescan } = useRescan(refetch);
  const actors = state.status === "ok" ? { status: "ok" as const, data: state.data.actors } : state;
  const count = state.status === "ok" ? state.data.actors.length : 0;

  const header =
    state.status === "ok" && state.data.anyVideos ? (
      <View style={styles.header}>
        <Text muted style={styles.lede}>
          {count === 0 ? "No one is named yet." : `${plural(count, "person", "people")} across your films.`}
        </Text>
        <Button
          label="All videos"
          variant="secondary"
          icon={{ sf: "square.grid.2x2", md: "grid_view" }}
          onPress={() => router.push("/videos")}
          style={styles.all}
        />
      </View>
    ) : undefined;

  const render = useCallback(
    (a: ActorCount) => (
      <CoverCard
        name={a.name}
        count={plural(a.videoCount, "video")}
        cover={a.cover}
        icon="person"
        onPress={() => router.push({ pathname: "/actor/[name]", params: { name: a.name } })}
      />
    ),
    [],
  );

  return (
    <GroupList
      state={actors}
      keyOf={(a) => a.name}
      render={render}
      aspectRatio={16 / 8}
      refreshing={scanning || refreshing}
      onRefresh={rescan}
      onRetry={refetch}
      header={header}
      empty={{
        title: "No videos yet",
        detail: "Put films in movies/ on the server, in a folder named after who's in them (movies/Keanu Reeves/The Matrix.mp4), then pull down to rescan.",
      }}
    />
  );
}

const styles = StyleSheet.create({
  header: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: space.md, marginBottom: space.lg },
  lede: { flex: 1 },
  all: { paddingHorizontal: space.md },
});
