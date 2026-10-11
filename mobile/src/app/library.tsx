import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { FlatList, ScrollView, StyleSheet, View } from "react-native";

import { listMedia, MediaKind, parseKind, type Media } from "@/api/media";
import { Chip } from "@/components/Chip";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { MediaCard } from "@/components/MediaCard";
import { useAsync } from "@/hooks/useAsync";

const KIND_FILTERS = [
  { label: "All", kind: undefined },
  { label: "Videos", kind: MediaKind.Video },
  { label: "Comics", kind: MediaKind.Comic },
];

const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });
const renderCard = ({ item }: { item: Media }) => (
  <View style={styles.cell}>
    <MediaCard media={item} onPress={openMedia} />
  </View>
);

// The filters live in the route, not in state: /library?kind=video&actor=Keanu+Reeves.
// Back restores the previous filter, and the URL is a deep link.
export default function Library() {
  const params = useLocalSearchParams<{ kind?: string; actor?: string }>();
  const kind = parseKind(params.kind);
  const actor = params.actor || undefined;

  // A new function only when kind or actor changes, and that is what starts a new request.
  // Leave actor out of the deps and the lint rule says so; otherwise tapping a second
  // actor would show the first one's list.
  const fetcher = useCallback((signal: AbortSignal) => listMedia({ kind, actor, signal }), [kind, actor]);
  const { state, refreshing, refetch } = useAsync(fetcher);

  return (
    <View style={styles.screen}>
      <Stack.Screen options={{ title: actor ?? "Library" }} />

      <ScrollView
        horizontal
        style={styles.filterBar}
        contentContainerStyle={styles.filters}
        showsHorizontalScrollIndicator={false}
      >
        {KIND_FILTERS.map((f) => (
          <Chip key={f.label} label={f.label} selected={kind === f.kind} onPress={() => router.setParams({ kind: f.kind })} />
        ))}
        {actor && <Chip label={`${actor}  ✕`} selected onPress={() => router.setParams({ actor: undefined })} />}
      </ScrollView>

      {state.status === "loading" && <Loading />}
      {state.status === "error" && <ErrorView error={state.error} onRetry={refetch} />}
      {state.status === "ok" && (
        <FlatList
          data={state.data}
          keyExtractor={(m) => m.id}
          renderItem={renderCard}
          numColumns={2}
          contentContainerStyle={styles.grid}
          refreshing={refreshing}
          onRefresh={refetch}
          ListEmptyComponent={<EmptyView {...emptyMessage(kind, actor)} />}
        />
      )}
    </View>
  );
}

function emptyMessage(kind: MediaKind | undefined, actor: string | undefined) {
  if (actor) return { title: `Nothing for ${actor}`, detail: kind ? "Try another kind, or clear the actor filter." : undefined };
  if (kind) return { title: kind === MediaKind.Video ? "No videos yet" : "No comics yet" };
  return { title: "No media yet", detail: "Add files under movies/ or comics/ and run a scan." };
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  filterBar: { flexGrow: 0 }, // a ScrollView grows by default and would take half the screen
  filters: { paddingHorizontal: 16, paddingVertical: 10, gap: 8 },
  grid: { paddingHorizontal: 10, paddingBottom: 24, flexGrow: 1 },
  cell: { width: "50%", padding: 6 },
});
