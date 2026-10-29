import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, ScrollView, StyleSheet, TextInput, View } from "react-native";

import { ApiError } from "@/api/client";
import { addActor, getMedia, MediaKind, removeActor, type Media } from "@/api/media";
import { ActorChip } from "@/components/ActorChip";
import { Button } from "@/components/Button";
import { ErrorView } from "@/components/ErrorView";
import { Icon } from "@/components/Icon";
import { Poster } from "@/components/Poster";
import { Skeleton } from "@/components/Skeleton";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { formatBytes, formatDate, formatDuration, formatResolution } from "@/lib/format";
import { resumeAt, resumePage } from "@/progress/resume";
import { onMedia, radius, space, touch, typography, useAppTheme } from "@/theme";

export default function Details() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback((signal: AbortSignal) => getMedia(id, { signal }), [id]);
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <DetailsSkeleton />;
  // A bad id comes back as a 404 ApiError; ErrorView says "Not found".
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  return <MediaDetails media={state.data} />;
}

function MediaDetails({ media: initial }: { media: Media }) {
  // The screen's own copy: adding or removing an actor replaces it with the server's answer.
  const [media, setMedia] = useState(initial);
  const isVideo = media.kind === MediaKind.Video;
  const actors = media.actors ?? []; // omitted from the JSON when there are none
  const open = () => router.push({ pathname: isVideo ? "/player/[id]" : "/reader/[id]", params: { id: media.id } });

  // The main action says where it will take you.
  const at = resumeAt(media);
  const page = resumePage(media);
  const action = isVideo
    ? at > 0
      ? `Resume from ${formatDuration(at)}`
      : "Play"
    : page > 0
      ? `Continue on page ${page + 1}`
      : "Read";

  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={styles.content}>
      <Stack.Screen options={{ title: media.title }} />

      {/* The poster is a second Play (or Read) button: a ▶ looks tappable. */}
      <Pressable
        onPress={open}
        accessibilityRole="button"
        accessibilityLabel={`${action}: ${media.title}`}
        style={({ pressed }) => [styles.posterBox, pressed && styles.pressed]}
      >
        <Poster media={media} style={styles.poster}>
          <View style={[styles.playBadge, { backgroundColor: onMedia.scrim }]}>
            {isVideo ? (
              <Icon sf="play.fill" md="play_arrow" size={28} color={onMedia.text} />
            ) : (
              <Icon sf="book.fill" md="menu_book" size={26} color={onMedia.text} />
            )}
          </View>
        </Poster>
      </Pressable>

      <View style={styles.heading}>
        <Text variant="display" accessibilityRole="header">
          {media.title}
        </Text>
        <Text variant="label" muted>
          {describeKind(media)} · {formatBytes(media.sizeBytes)} · added {formatDate(media.addedAt)}
        </Text>
      </View>

      <Button label={action} onPress={open} haptic />

      {isVideo && (
        <View style={styles.section}>
          <Text variant="title" accessibilityRole="header">
            Actors
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
          <AddActor mediaId={media.id} onAdded={setMedia} />
        </View>
      )}
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
  const { colors } = useAppTheme();
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
        setError(err instanceof ApiError ? err.message : "Couldn't reach the server. Check your Wi-Fi and try again.");
        setBusy(false);
      },
    );
  };

  return (
    <View style={styles.addActor}>
      <View style={styles.addRow}>
        <TextInput
          value={name}
          onChangeText={setName}
          onSubmitEditing={submit}
          placeholder="Add someone who's in it"
          placeholderTextColor={colors.textMuted}
          returnKeyType="done"
          autoCapitalize="words"
          autoCorrect={false}
          accessibilityLabel="Actor name"
          style={[styles.input, { color: colors.text, backgroundColor: colors.surface }]}
        />
        <Button label="Add" onPress={submit} disabled={busy || name.trim() === ""} variant="secondary" />
      </View>
      {error && (
        <Text variant="caption" color={colors.danger} accessibilityLiveRegion="polite">
          {error}
        </Text>
      )}
    </View>
  );
}

// DetailsSkeleton has the screen's shape: poster, title, meta line, button.
function DetailsSkeleton() {
  return (
    <View style={styles.content} accessibilityLabel="Loading">
      <Skeleton aspectRatio={16 / 9} style={styles.posterBox} />
      <View style={styles.heading}>
        <Skeleton width="60%" height={30} />
        <Skeleton width="80%" height={16} />
      </View>
      <Skeleton height={touch.min} />
    </View>
  );
}

const styles = StyleSheet.create({
  content: { padding: space.lg, gap: space.xl },
  posterBox: { alignSelf: "center", width: "100%", maxWidth: 480 },
  poster: { borderRadius: radius.lg, maxHeight: 360 },
  playBadge: { position: "absolute", width: 64, height: 64, borderRadius: 32, alignItems: "center", justifyContent: "center" },
  pressed: { opacity: 0.7 },
  heading: { gap: space.xs },
  section: { gap: space.md },
  actors: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  addActor: { gap: space.xs },
  addRow: { flexDirection: "row", gap: space.sm },
  input: { ...typography.body, flex: 1, borderRadius: radius.lg, paddingHorizontal: space.md, minHeight: touch.min },
});
