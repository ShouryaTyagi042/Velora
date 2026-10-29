import { router, Stack, useLocalSearchParams, useTheme } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, TextInput, View } from "react-native";

import { ApiError } from "@/api/client";
import { addActor, getMedia, MediaKind, removeActor, type Media } from "@/api/media";
import { ActorChip } from "@/components/ActorChip";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { Poster } from "@/components/Poster";
import { useAsync } from "@/hooks/useAsync";
import { formatBytes, formatDate, formatDuration, formatResolution } from "@/lib/format";

export default function Details() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback((signal: AbortSignal) => getMedia(id, { signal }), [id]);
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <Loading />;
  // A bad id comes back as a 404 ApiError; ErrorView says "Not found".
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  return <MediaDetails media={state.data} />;
}

function MediaDetails({ media: initial }: { media: Media }) {
  const { colors } = useTheme();
  // The screen's own copy: adding or removing an actor replaces it with the server's answer.
  const [media, setMedia] = useState(initial);
  const isVideo = media.kind === MediaKind.Video;
  const actors = media.actors ?? []; // omitted from the JSON when there are none
  const open = () => router.push({ pathname: isVideo ? "/player/[id]" : "/reader/[id]", params: { id: media.id } });

  return (
    <ScrollView contentContainerStyle={styles.content}>
      <Stack.Screen options={{ title: media.title }} />

      {/* The poster is a second Play (or Read) button: a ▶ looks tappable. */}
      <Pressable
        onPress={open}
        accessibilityRole="button"
        accessibilityLabel={`${isVideo ? "Play" : "Read"} ${media.title}`}
        style={({ pressed }) => [styles.posterBox, pressed && styles.pressed]}
      >
        <Poster media={media} style={styles.poster} />
      </Pressable>

      <Text style={[styles.title, { color: colors.text }]}>{media.title}</Text>
      <Text style={styles.meta}>
        {describeKind(media)} · {formatBytes(media.sizeBytes)} · added {formatDate(media.addedAt)}
      </Text>

      {actors.length > 0 && (
        <View style={styles.actors}>
          {actors.map((tag) => (
            <ActorChip
              key={tag.name}
              tag={tag}
              onRemove={() =>
                removeActor(media.id, tag.name).then(
                  () => setMedia((m) => ({ ...m, actors: m.actors?.filter((a) => a.name !== tag.name) })),
                  () => {}, // the chip stays; a failed remove changes nothing
                )
              }
            />
          ))}
        </View>
      )}
      {isVideo && <AddActor mediaId={media.id} onAdded={setMedia} />}

      <Pressable
        style={({ pressed }) => [styles.button, { backgroundColor: colors.primary }, pressed && styles.pressed]}
        onPress={open}
      >
        <Text style={styles.buttonText}>{isVideo ? "Play" : "Read"}</Text>
      </Pressable>
    </ScrollView>
  );
}

// describeKind is the first part of the meta line: "1:42:10 · 1080p · h264", "Comic · 24 pages".
function describeKind(m: Media): string {
  if (m.kind === MediaKind.Comic) return `Comic · ${m.comic?.pageCount ?? 0} pages`;
  if (!m.video) return "Video"; // still being processed
  const v = m.video;
  return `${formatDuration(v.durationMs / 1000)} · ${formatResolution(v.width, v.height)} · ${v.codec}`;
}

// AddActor is a one-line form: type a name, submit, and the video comes back with it.
function AddActor({ mediaId, onAdded }: { mediaId: string; onAdded: (m: Media) => void }) {
  const { colors } = useTheme();
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    addActor(mediaId, name).then(
      (m) => {
        onAdded(m);
        setName("");
        setBusy(false);
      },
      (err: unknown) => {
        // The server's message is written for people ("name must be non-empty"); show it.
        setError(err instanceof ApiError ? err.message : "Can't reach the server.");
        setBusy(false);
      },
    );
  };
  const disabled = busy || name.trim() === "";

  return (
    <View style={styles.addActor}>
      <View style={styles.addRow}>
        <TextInput
          value={name}
          onChangeText={setName}
          onSubmitEditing={submit}
          placeholder="Add actor"
          placeholderTextColor="#888"
          returnKeyType="done"
          autoCapitalize="words"
          autoCorrect={false}
          accessibilityLabel="Actor name"
          style={[styles.input, { color: colors.text, borderColor: colors.border }]}
        />
        <Pressable
          onPress={submit}
          disabled={disabled}
          style={({ pressed }) => [styles.addButton, { backgroundColor: colors.primary }, (pressed || disabled) && styles.disabled]}
        >
          <Text style={styles.buttonText}>Add</Text>
        </Pressable>
      </View>
      {error && <Text style={styles.error}>{error}</Text>}
    </View>
  );
}

const styles = StyleSheet.create({
  content: { padding: 16, gap: 12 },
  posterBox: { alignSelf: "center", width: "100%", maxWidth: 480 },
  poster: { borderRadius: 10, maxHeight: 360 },
  title: { fontSize: 22, fontWeight: "700" },
  meta: { color: "#888" },
  actors: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  button: { marginTop: 8, padding: 14, borderRadius: 10, alignItems: "center" },
  pressed: { opacity: 0.7 },
  disabled: { opacity: 0.5 },
  buttonText: { color: "#fff", fontWeight: "600", fontSize: 16 },
  addActor: { gap: 4 },
  addRow: { flexDirection: "row", gap: 8 },
  input: { flex: 1, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, minHeight: 44, fontSize: 16 },
  addButton: { paddingHorizontal: 16, borderRadius: 10, alignItems: "center", justifyContent: "center" },
  error: { color: "#d33", fontSize: 13 },
});
