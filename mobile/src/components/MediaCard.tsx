import { Pressable, StyleSheet, View, type ViewStyle } from "react-native";

import { MediaKind, type Media, type Progress } from "@/api/media";
import { formatDuration } from "@/lib/format";
import { onMedia, radius, space, useAppTheme } from "@/theme";
import { Poster } from "./Poster";
import { Text } from "./Text";

// showKind draws the VIDEO / COMIC badge: useful where both kinds mix (Home, Search), noise on a
// screen that only shows one.
type Props = { media: Media; onPress: (media: Media) => void; style?: ViewStyle; showKind?: boolean };

// MediaCard is a poster and a title. A video's poster shows its duration once it's known, and
// anything started shows a thin progress bar along the poster's bottom edge. One accessible
// element: a screen reader reads "The Matrix, video, 2 hours 16 minutes, 40% watched".
export function MediaCard({ media, onPress, style, showKind = true }: Props) {
  const { colors } = useAppTheme();
  const isVideo = media.kind === MediaKind.Video;
  const done = media.progress && !media.progress.completed ? fraction(media.progress) : null;
  return (
    <Pressable
      onPress={() => onPress(media)}
      accessibilityRole="button"
      accessibilityLabel={describe(media, done)}
      style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}
    >
      <Poster media={media}>
        {showKind && (
          <Text variant="caption" color={onMedia.text} style={[styles.badge, styles.kind]}>
            {isVideo ? "VIDEO" : "COMIC"}
          </Text>
        )}
        {media.video && (
          <Text variant="caption" color={onMedia.text} style={[styles.badge, styles.duration]}>
            {formatDuration(media.video.durationMs / 1000)}
          </Text>
        )}
        {done !== null && (
          <View style={[styles.track, { backgroundColor: onMedia.track }]}>
            <View style={[styles.bar, { width: `${done * 100}%`, backgroundColor: colors.primary }]} />
          </View>
        )}
      </Poster>
      <Text variant="label" numberOfLines={2}>
        {media.title}
      </Text>
    </Pressable>
  );
}

// fraction is how far through an item its progress is, 0 to 1.
function fraction(p: Progress): number {
  if (p.durationMs) return Math.min(1, (p.positionMs ?? 0) / p.durationMs);
  if (p.pageCount) return Math.min(1, ((p.page ?? 0) + 1) / p.pageCount);
  return 0;
}

function describe(m: Media, done: number | null): string {
  const parts = [m.title, m.kind === MediaKind.Video ? "video" : "comic"];
  if (m.video) parts.push(formatDuration(m.video.durationMs / 1000));
  if (done !== null) parts.push(`${Math.round(done * 100)}% ${m.kind === MediaKind.Video ? "watched" : "read"}`);
  return parts.join(", ");
}

const styles = StyleSheet.create({
  card: { gap: space.sm },
  pressed: { opacity: 0.6 },
  badge: {
    position: "absolute",
    fontWeight: "700",
    backgroundColor: onMedia.scrim,
    paddingHorizontal: space.xs,
    borderRadius: radius.sm,
    overflow: "hidden",
  },
  kind: { top: space.sm, left: space.sm },
  duration: { bottom: space.sm, right: space.sm, fontVariant: ["tabular-nums"] },
  track: { position: "absolute", left: 0, right: 0, bottom: 0, height: 4 },
  bar: { height: "100%" },
});
