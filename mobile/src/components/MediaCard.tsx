import { useTheme } from "expo-router";
import { Pressable, StyleSheet, Text, View, type ViewStyle } from "react-native";

import { MediaKind, type Media, type Progress } from "@/api/media";
import { formatDuration } from "@/lib/format";
import { Poster } from "./Poster";

type Props = { media: Media; onPress: (media: Media) => void; style?: ViewStyle };

// MediaCard is a poster and a title. A video's poster shows its duration once it's known, and
// anything started shows a thin progress bar along the poster's bottom edge.
export function MediaCard({ media, onPress, style }: Props) {
  const { colors } = useTheme();
  const isVideo = media.kind === MediaKind.Video;
  const done = media.progress && !media.progress.completed ? fraction(media.progress) : null;
  return (
    <Pressable onPress={() => onPress(media)} style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}>
      <Poster media={media}>
        <Text style={[styles.badge, styles.kind]}>{isVideo ? "VIDEO" : "COMIC"}</Text>
        {media.video && (
          <Text style={[styles.badge, styles.duration]}>{formatDuration(media.video.durationMs / 1000)}</Text>
        )}
        {done !== null && (
          <View style={styles.track}>
            <View style={[styles.bar, { width: `${done * 100}%`, backgroundColor: colors.primary }]} />
          </View>
        )}
      </Poster>
      <Text numberOfLines={2} style={[styles.title, { color: colors.text }]}>
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

const styles = StyleSheet.create({
  card: { gap: 6 },
  pressed: { opacity: 0.6 },
  badge: {
    position: "absolute",
    color: "#fff",
    fontSize: 10,
    fontWeight: "700",
    backgroundColor: "#00000066",
    paddingHorizontal: 5,
    paddingVertical: 2,
    borderRadius: 4,
    overflow: "hidden",
  },
  kind: { top: 6, left: 6 },
  duration: { bottom: 6, right: 6, fontVariant: ["tabular-nums"] },
  track: { position: "absolute", left: 0, right: 0, bottom: 0, height: 4, backgroundColor: "#ffffff59" },
  bar: { height: "100%" },
  title: { fontSize: 14, fontWeight: "500" },
});
