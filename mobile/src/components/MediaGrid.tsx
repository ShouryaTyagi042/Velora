import { router } from "expo-router";
import { FlatList, StyleSheet, View } from "react-native";

import type { Media } from "@/api/media";
import type { Async } from "@/hooks/useAsync";
import { space } from "@/theme";
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
};

// MediaGrid is a library screen's body: a grid of cards that scrolls under the glass header,
// skeleton cards while loading, an empty state, and pull to refresh.
export function MediaGrid({ state, columns, aspectRatio, refreshing, onRefresh, onRetry, empty, header }: Props) {
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
});
