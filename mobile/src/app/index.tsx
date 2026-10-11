import { router, useTheme } from "expo-router";
import { FlatList, Pressable, ScrollView, StyleSheet, Text } from "react-native";

import { listMedia, type Media } from "@/api/media";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { MediaCard } from "@/components/MediaCard";
import { useAsync } from "@/hooks/useAsync";

const RECENT_COUNT = 10;

// Defined outside the component: the same function every render, so neither the list
// nor its rows re-render just because Home did.
const fetchAll = (signal: AbortSignal) => listMedia({ signal });
const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });
const renderRecent = ({ item }: { item: Media }) => (
  <MediaCard media={item} onPress={openMedia} style={styles.recentCard} />
);

export default function Home() {
  const { colors } = useTheme();
  const { state, refetch } = useAsync(fetchAll);

  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;

  // The server sorts by title; "recently added" is the newest addedAt first.
  const recent = [...state.data].sort((a, b) => b.addedAt.localeCompare(a.addedAt)).slice(0, RECENT_COUNT);

  return (
    <ScrollView contentContainerStyle={styles.content}>
      <Text style={[styles.heading, { color: colors.text }]}>Continue watching</Text>
      <Text style={styles.muted}>Nothing yet. Progress tracking arrives in phase 10.</Text>

      <Text style={[styles.heading, { color: colors.text }]}>Recently added</Text>
      {recent.length === 0 ? (
        <EmptyView title="No media yet" detail="Add files under movies/ or comics/ and run a scan." />
      ) : (
        <FlatList
          horizontal
          data={recent}
          keyExtractor={(m) => m.id}
          renderItem={renderRecent}
          contentContainerStyle={styles.recentRow}
          showsHorizontalScrollIndicator={false}
        />
      )}

      <Pressable
        style={({ pressed }) => [styles.button, { backgroundColor: colors.primary }, pressed && { opacity: 0.7 }]}
        onPress={() => router.push("/library")}
      >
        <Text style={styles.buttonText}>Open the library ({state.data.length})</Text>
      </Pressable>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  content: { paddingVertical: 16, gap: 12 },
  heading: { fontSize: 20, fontWeight: "700", paddingHorizontal: 16 },
  muted: { color: "#888", paddingHorizontal: 16 },
  recentRow: { paddingHorizontal: 16, gap: 12 },
  recentCard: { width: 150 },
  button: { marginHorizontal: 16, marginTop: 12, padding: 14, borderRadius: 10, alignItems: "center" },
  buttonText: { color: "#fff", fontWeight: "600", fontSize: 16 },
});
