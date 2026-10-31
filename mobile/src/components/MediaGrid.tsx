import { router } from "expo-router";
import { ActivityIndicator, FlatList, StyleSheet, View } from "react-native";

import type { Media } from "@/api/media";
import type { Async } from "@/hooks/useAsync";
import { space } from "@/theme";
import { Button } from "./Button";
import { EmptyView } from "./EmptyView";
import { ErrorView } from "./ErrorView";
import { MediaCard } from "./MediaCard";
import { Skeleton } from "./Skeleton";

const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });

type Props = {
  state: Async<Media[]>;
  columns: number;
  aspectRatio: number; // the posters' shape, for the skeleton
  refreshing: boolean;
  onRefresh: () => void;
  onRetry: () => void;
  empty: { title: string; detail?: string };
  header?: React.ReactElement;
  // Paging (usePages): the next page loads as the end of the list comes into view.
  onEndReached?: () => void;
  loadingMore?: boolean;
  moreError?: Error | null;
};

// MediaGrid is a library screen's body: a grid of cards that scrolls under the glass header,
// skeleton cards while loading, an empty state, pull to refresh, and more pages as you scroll.
export function MediaGrid({
  state,
  columns,
  aspectRatio,
  refreshing,
  onRefresh,
  onRetry,
  empty,
  header,
  onEndReached,
  loadingMore = false,
  moreError = null,
}: Props) {
  if (state.status === "error") return <ErrorView error={state.error} onRetry={onRetry} />;

  const cell = { width: `${100 / columns}%` } as const;
  return (
    <FlatList
      // A new key when the column count changes: FlatList can't change numColumns in place.
      key={columns}
      contentInsetAdjustmentBehavior="automatic"
      data={state.status === "ok" ? state.data : []}
      keyExtractor={(m) => m.id}
      renderItem={({ item }) => (
        <View style={[styles.cell, cell]}>
          <MediaCard media={item} onPress={openMedia} showKind={false} />
        </View>
      )}
      numColumns={columns}
      ListHeaderComponent={header}
      contentContainerStyle={styles.grid}
      refreshing={refreshing}
      onRefresh={onRefresh}
      // Half a screen before the end: the next page is usually there before you reach it.
      onEndReached={onEndReached}
      onEndReachedThreshold={0.5}
      ListFooterComponent={
        loadingMore ? (
          <ActivityIndicator style={styles.footer} accessibilityLabel="Loading more" />
        ) : moreError && onEndReached ? (
          // onEndReached won't fire again for content that hasn't grown, so retry is a button.
          <Button label="Couldn't load more. Try again" variant="secondary" onPress={onEndReached} style={styles.footer} />
        ) : undefined
      }
      ListEmptyComponent={
        state.status === "loading" ? (
          <View style={styles.skeletons} accessibilityLabel="Loading">
            {Array.from({ length: columns * 2 }, (_, i) => (
              <View key={i} style={[styles.cell, cell]}>
                <Skeleton aspectRatio={aspectRatio} />
                <Skeleton width="70%" height={16} style={styles.skeletonTitle} />
              </View>
            ))}
          </View>
        ) : (
          <EmptyView {...empty} />
        )
      }
    />
  );
}

const styles = StyleSheet.create({
  grid: { paddingHorizontal: space.sm, paddingBottom: space.xl, flexGrow: 1 },
  cell: { padding: space.sm - 2 },
  skeletons: { flexDirection: "row", flexWrap: "wrap" },
  skeletonTitle: { marginTop: space.sm },
  footer: { margin: space.lg },
});
