import { FlatList, StyleSheet, View } from "react-native";

import type { Async } from "@/hooks/useAsync";
import { space } from "@/theme";
import { EmptyView } from "./EmptyView";
import { ErrorView } from "./ErrorView";
import { Skeleton } from "./Skeleton";

type Props<T> = {
  state: Async<T[]>;
  keyOf: (item: T) => string;
  render: (item: T) => React.ReactElement;
  aspectRatio: number; // the cards' shape, for the skeleton
  refreshing: boolean;
  onRefresh: () => void;
  onRetry: () => void;
  empty: { title: string; detail?: string };
  header?: React.ReactElement;
  onEndReached?: () => void;
  footer?: React.ReactElement;
};

// GroupList is a single-column screen body: one full-width card per row, under the large title,
// with skeleton cards while loading, an empty state and pull to refresh. Videos and anime are
// always one column: a 16:9 frame needs the whole width to be legible on a phone.
export function GroupList<T>({ state, keyOf, render, aspectRatio, refreshing, onRefresh, onRetry, empty, header, onEndReached, footer }: Props<T>) {
  if (state.status === "error") return <ErrorView error={state.error} onRetry={onRetry} />;
  return (
    <FlatList
      contentInsetAdjustmentBehavior="automatic"
      data={state.status === "ok" ? state.data : []}
      keyExtractor={keyOf}
      renderItem={({ item }) => render(item)}
      ListHeaderComponent={header}
      ListFooterComponent={footer}
      contentContainerStyle={styles.list}
      ItemSeparatorComponent={Separator}
      refreshing={refreshing}
      onRefresh={onRefresh}
      onEndReached={onEndReached}
      onEndReachedThreshold={0.6}
      ListEmptyComponent={
        state.status === "loading" ? (
          <View style={styles.skeletons} accessibilityLabel="Loading">
            {[0, 1, 2].map((i) => (
              <View key={i} style={styles.skeleton}>
                <Skeleton aspectRatio={aspectRatio} />
                <Skeleton width="45%" height={14} />
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

function Separator() {
  return <View style={styles.separator} />;
}

const styles = StyleSheet.create({
  list: { paddingHorizontal: space.lg, paddingBottom: space.xxxl, flexGrow: 1 },
  separator: { height: space.xl },
  skeletons: { gap: space.xl },
  skeleton: { gap: space.sm },
});
