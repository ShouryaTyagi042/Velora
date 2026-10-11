import { Stack, useLocalSearchParams, useTheme } from "expo-router";
import { useCallback } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";

import { getMedia, MediaKind, type Media } from "@/api/media";
import { ActorChip } from "@/components/ActorChip";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { useAsync } from "@/hooks/useAsync";
import { formatBytes, formatDate } from "@/lib/format";

export default function Details() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback((signal: AbortSignal) => getMedia(id, { signal }), [id]);
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <Loading />;
  // A bad id comes back as a 404 ApiError; ErrorView says "Not found".
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  return <MediaDetails media={state.data} />;
}

function MediaDetails({ media }: { media: Media }) {
  const { colors } = useTheme();
  const isVideo = media.kind === MediaKind.Video;
  const actors = media.actors ?? []; // omitted from the JSON when there are none

  return (
    <ScrollView contentContainerStyle={styles.content}>
      <Stack.Screen options={{ title: media.title }} />

      <View style={[styles.poster, { aspectRatio: isVideo ? 16 / 9 : 2 / 3 }]}>
        <Text style={styles.glyph}>{isVideo ? "▶" : "▤"}</Text>
      </View>

      <Text style={[styles.title, { color: colors.text }]}>{media.title}</Text>
      <Text style={styles.meta}>
        {isVideo ? "Video" : `Comic · ${media.comic?.pageCount ?? 0} pages`} · {formatBytes(media.sizeBytes)} · added{" "}
        {formatDate(media.addedAt)}
      </Text>

      {actors.length > 0 && (
        <View style={styles.actors}>
          {actors.map((tag) => (
            <ActorChip key={tag.name} tag={tag} />
          ))}
        </View>
      )}

      {/* No-op until the player (phase 6) and the comic reader (phase 8) exist. */}
      <View style={[styles.button, { backgroundColor: colors.primary }]}>
        <Text style={styles.buttonText}>{isVideo ? "Play" : "Read"}</Text>
      </View>
      <Text style={styles.hint}>{isVideo ? "The player arrives in phase 6." : "The reader arrives in phase 8."}</Text>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  content: { padding: 16, gap: 12 },
  poster: {
    backgroundColor: "#2b3a55",
    borderRadius: 10,
    alignItems: "center",
    justifyContent: "center",
    maxHeight: 360,
    alignSelf: "center",
    width: "100%",
  },
  glyph: { color: "#ffffffcc", fontSize: 40 },
  title: { fontSize: 22, fontWeight: "700" },
  meta: { color: "#888" },
  actors: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  button: { marginTop: 8, padding: 14, borderRadius: 10, alignItems: "center", opacity: 0.5 },
  buttonText: { color: "#fff", fontWeight: "600", fontSize: 16 },
  hint: { color: "#888", textAlign: "center", fontSize: 13 },
});
