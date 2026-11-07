import * as Haptics from "expo-haptics";
import { Pressable, StyleSheet, View } from "react-native";

import type { Media } from "@/api/media";
import { formatDuration } from "@/lib/format";
import { PreviewVideo } from "@/preview/PreviewVideo";
import { startPreview, useIsPreviewing } from "@/preview/store";
import { onMedia, radius, space, useAppTheme } from "@/theme";
import { Icon } from "./Icon";
import { Poster } from "./Poster";
import { Text } from "./Text";

type Props = { episode: Media; onPress: (m: Media) => void; current?: boolean };

// EpisodeRow is one episode in a series' list: a still on the left, its title and running time on
// the right, how far you got along the bottom of the still. current marks the episode Play would
// start (the one you're in the middle of), with a small play glyph over its still.
export function EpisodeRow({ episode, onPress, current = false }: Props) {
  const { colors } = useAppTheme();
  const previewing = useIsPreviewing(episode.id);
  const canPreview = episode.video?.preview === true;
  const p = episode.progress;
  const done = p && p.durationMs ? (p.completed ? 1 : Math.min(1, (p.positionMs ?? 0) / p.durationMs)) : null;
  const preview = () => {
    void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
    startPreview(episode);
  };

  return (
    <Pressable
      onPress={() => onPress(episode)}
      onLongPress={canPreview ? preview : undefined}
      delayLongPress={350}
      accessibilityRole="button"
      accessibilityLabel={[
        episode.title,
        episode.video && formatDuration(episode.video.durationMs / 1000),
        p?.completed ? "watched" : done !== null ? `${Math.round(done * 100)}% watched` : null,
      ]
        .filter(Boolean)
        .join(", ")}
      style={({ pressed }) => [styles.row, pressed && { backgroundColor: colors.surface }]}
    >
      <Poster media={episode} style={styles.still}>
        {previewing && <PreviewVideo id={episode.id} />}
        {current && !previewing && (
          <View style={[styles.glyph, { backgroundColor: onMedia.scrim }]}>
            <Icon sf="play.fill" md="play_arrow" size={16} color={onMedia.text} />
          </View>
        )}
        {done !== null && (
          <View style={[styles.track, { backgroundColor: onMedia.track }]}>
            <View style={[styles.bar, { width: `${done * 100}%`, backgroundColor: colors.primary }]} />
          </View>
        )}
      </Poster>
      <View style={styles.text}>
        <Text variant="label" numberOfLines={2}>
          {episode.title}
        </Text>
        <Text variant="caption" muted>
          {episode.video ? formatDuration(episode.video.durationMs / 1000) : "Still processing"}
          {p?.completed ? "  ·  Watched" : ""}
        </Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: space.md, borderRadius: radius.md, marginHorizontal: -space.xs, padding: space.xs },
  still: { width: "42%" },
  text: { flex: 1, gap: space.xs },
  glyph: { width: 34, height: 34, borderRadius: 17, alignItems: "center", justifyContent: "center" },
  track: { position: "absolute", left: 0, right: 0, bottom: 0, height: 3 },
  bar: { height: "100%" },
});
