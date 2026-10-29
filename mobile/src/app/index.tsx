import { router, useFocusEffect, useTheme } from "expo-router";
import { useCallback } from "react";
import { FlatList, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { listMedia, MediaKind, type Media } from "@/api/media";
import { getContinue } from "@/api/progress";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { MediaCard } from "@/components/MediaCard";
import { useAsync } from "@/hooks/useAsync";

const RECENT_COUNT = 10;

// Defined outside the component: the same function every render, so neither the list
// nor its rows re-render just because Home did.
const fetchHome = async (signal: AbortSignal) => {
  const [all, cont] = await Promise.all([listMedia({ signal }), getContinue({ signal })]);
  return { all, ...cont };
};
const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });
const resume = (m: Media) =>
  router.push({ pathname: m.kind === MediaKind.Video ? "/player/[id]" : "/reader/[id]", params: { id: m.id } });
const renderRecent = ({ item }: { item: Media }) => (
  <MediaCard media={item} onPress={openMedia} style={styles.recentCard} />
);
const renderResume = ({ item }: { item: Media }) => (
  <MediaCard media={item} onPress={resume} style={styles.recentCard} />
);

export default function Home() {
  const { colors } = useTheme();
  const { state, refetch } = useAsync(fetchHome);

  // Coming back from the player or the reader: the Continue rows have changed.
  useFocusEffect(useCallback(() => refetch(), [refetch]));

  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;

  // The server sorts by title; "recently added" is the newest addedAt first.
  const { all, watching, reading } = state.data;
  const recent = [...all].sort((a, b) => Date.parse(b.addedAt) - Date.parse(a.addedAt)).slice(0, RECENT_COUNT);

  return (
    <ScrollView contentContainerStyle={styles.content}>
      {[
        { title: "Continue watching", items: watching },
        { title: "Continue reading", items: reading },
      ].map(
        (row) =>
          row.items.length > 0 && (
            <View key={row.title} style={styles.section}>
              <Text style={[styles.heading, { color: colors.text }]}>{row.title}</Text>
              <FlatList
                horizontal
                data={row.items}
                keyExtractor={(m) => m.id}
                renderItem={renderResume}
                contentContainerStyle={styles.recentRow}
                showsHorizontalScrollIndicator={false}
              />
            </View>
          ),
      )}

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
        <Text style={styles.buttonText}>Open the library ({all.length})</Text>
      </Pressable>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  content: { paddingVertical: 16, gap: 12 },
  heading: { fontSize: 20, fontWeight: "700", paddingHorizontal: 16 },
  section: { gap: 12 },
  recentRow: { paddingHorizontal: 16, gap: 12 },
  recentCard: { width: 150 },
  button: { marginHorizontal: 16, marginTop: 12, padding: 14, borderRadius: 10, alignItems: "center" },
  buttonText: { color: "#fff", fontWeight: "600", fontSize: 16 },
});
