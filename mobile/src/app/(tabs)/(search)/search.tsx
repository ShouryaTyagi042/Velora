import { router, Stack } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { ActivityIndicator, FlatList, StyleSheet, View } from "react-native";

import { listMedia, type Media, type Page } from "@/api/media";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { MediaCard } from "@/components/MediaCard";
import { usePages } from "@/hooks/usePages";
import { space } from "@/theme";

const nothing: Page<Media> = { items: [], nextCursor: null };
const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });
const renderCard = ({ item }: { item: Media }) => (
  <View style={styles.cell}>
    <MediaCard media={item} onPress={openMedia} />
  </View>
);

// SEARCH_DELAY is how long typing must pause before a search goes out: one request per word,
// not one per keystroke.
const SEARCH_DELAY = 300;

// Search: the native search field (in the glass tab bar on iOS 26, in the header elsewhere).
// The server matches titles and actors' names, ignoring case and accents. It used to happen here,
// over the whole library downloaded up front; at 100k items that download was 33 MB (phase 12).
export default function Search() {
  const [query, setQuery] = useState("");
  const [q, setQ] = useState(""); // the query, once typing pauses
  useEffect(() => {
    const t = setTimeout(() => setQ(query.trim()), SEARCH_DELAY);
    return () => clearTimeout(t);
  }, [query]);

  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => (q ? listMedia({ q, after, signal }) : Promise.resolve(nothing)),
    [q],
  );
  const { state, refetch, loadMore, loadingMore } = usePages(fetchPage);

  const header = (
    <Stack.Screen
      options={{
        headerSearchBarOptions: {
          placeholder: "Titles and actors",
          onChangeText: (e) => setQuery(e.nativeEvent.text),
          autoCapitalize: "none",
        },
      }}
    />
  );

  if (state.status === "loading") return <>{header}<Loading /></>;
  if (state.status === "error") return <>{header}<ErrorView error={state.error} onRetry={refetch} /></>;

  return (
    <>
      {header}
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        keyboardDismissMode="on-drag"
        data={state.data}
        keyExtractor={(m) => m.id}
        renderItem={renderCard}
        numColumns={2}
        contentContainerStyle={styles.grid}
        onEndReached={loadMore}
        onEndReachedThreshold={0.5}
        ListFooterComponent={loadingMore ? <ActivityIndicator style={styles.footer} accessibilityLabel="Loading more" /> : undefined}
        ListEmptyComponent={
          q ? (
            <EmptyView title={`Nothing matches “${q}”`} detail="Search looks at titles and actors' names. Check the spelling, or try part of a name." />
          ) : (
            <EmptyView title="Search your library" detail="Find a film, a comic or an actor by any part of its name." />
          )
        }
      />
    </>
  );
}

const styles = StyleSheet.create({
  grid: { paddingHorizontal: space.sm, paddingBottom: space.xl, flexGrow: 1 },
  cell: { width: "50%", padding: space.sm - 2 },
  footer: { margin: space.lg },
});
