import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, FlatList, StyleSheet, View } from "react-native";

import { listMedia, MediaKind, type Media } from "@/api/media";
import { Button } from "@/components/Button";
import { EmptyView } from "@/components/EmptyView";
import { EpisodeRow } from "@/components/EpisodeRow";
import { ErrorView } from "@/components/ErrorView";
import { Gradient } from "@/components/Gradient";
import { Poster } from "@/components/Poster";
import { Skeleton } from "@/components/Skeleton";
import { Text } from "@/components/Text";
import { usePages } from "@/hooks/usePages";
import { useRefetchOnLibraryChange } from "@/hooks/useLibraryChanges";
import { useRescan } from "@/hooks/useRescan";
import { resumeAt } from "@/progress/resume";
import { gradients, radius, space } from "@/theme";

const play = (m: Media) => router.push({ pathname: "/player/[id]", params: { id: m.id } });
const details = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });

// A series: its first episode's still, Play (or Resume, on the episode you're in the middle of),
// then every episode in natural order (2 before 10), one per row. Tapping an episode opens its
// details; the button plays.
export default function Series() {
  const { name } = useLocalSearchParams<{ name: string }>();
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => listMedia({ kind: MediaKind.Video, series: name, after, signal }),
    [name],
  );
  const { state, refreshing, refetch, loadMore, loadingMore } = usePages(fetchPage);
  const { scanning, rescan } = useRescan(refetch);
  useRefetchOnLibraryChange(refetch);

  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  const episodes = state.status === "ok" ? state.data : [];
  const next = upNext(episodes);

  return (
    <>
      <Stack.Screen options={{ title: name }} />
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        data={episodes}
        keyExtractor={(m) => m.id}
        renderItem={({ item }) => <EpisodeRow episode={item} onPress={details} current={item.id === next?.id} />}
        contentContainerStyle={styles.list}
        ItemSeparatorComponent={() => <View style={styles.separator} />}
        refreshing={scanning || refreshing}
        onRefresh={rescan}
        onEndReached={loadMore}
        onEndReachedThreshold={0.6}
        ListHeaderComponent={
          state.status === "loading" ? (
            <View style={styles.header} accessibilityLabel="Loading">
              <Skeleton aspectRatio={16 / 9} />
              <Skeleton height={44} />
            </View>
          ) : episodes.length > 0 && next ? (
            <View style={styles.header}>
              <Poster media={episodes[0]} style={styles.cover}>
                <Gradient colors={gradients.posterBottom} style={styles.fade} />
              </Poster>
              <Button
                label={resumeAt(next) > 0 ? `Resume ${next.title}` : next === episodes[0] ? "Play" : `Play ${next.title}`}
                icon={{ sf: "play.fill", md: "play_arrow" }}
                variant="play"
                haptic
                onPress={() => play(next)}
              />
              <Text variant="title" style={styles.heading}>
                Episodes
              </Text>
            </View>
          ) : undefined
        }
        ListFooterComponent={loadingMore ? <ActivityIndicator style={styles.footer} /> : undefined}
        ListEmptyComponent={
          state.status === "ok" ? (
            <EmptyView title={`No episodes of ${name}`} detail={`Put episodes in anime/${name}/ on the server and pull down to rescan.`} />
          ) : undefined
        }
      />
    </>
  );
}

// upNext is the episode Play starts: the one you're partway through, else the first you haven't
// finished, else (all watched) the first.
function upNext(episodes: Media[]): Media | undefined {
  return (
    episodes.find((e) => e.progress && !e.progress.completed && resumeAt(e) > 0) ??
    episodes.find((e) => !e.progress?.completed) ??
    episodes[0]
  );
}

const styles = StyleSheet.create({
  list: { paddingHorizontal: space.lg, paddingBottom: space.xxxl, flexGrow: 1 },
  header: { gap: space.lg, marginBottom: space.lg },
  cover: { borderRadius: radius.md },
  fade: { top: "60%" },
  heading: { marginTop: space.sm },
  separator: { height: space.md },
  footer: { margin: space.lg },
});
