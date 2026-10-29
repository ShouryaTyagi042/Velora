import { useTheme } from "expo-router";
import { Pressable, StyleSheet, Text, type ViewStyle } from "react-native";

import { MediaKind, type Media } from "@/api/media";
import { formatDuration } from "@/lib/format";
import { Poster } from "./Poster";

type Props = { media: Media; onPress: (media: Media) => void; style?: ViewStyle };

// MediaCard is a poster and a title. A video's poster shows its duration once it's known.
export function MediaCard({ media, onPress, style }: Props) {
  const { colors } = useTheme();
  const isVideo = media.kind === MediaKind.Video;
  return (
    <Pressable onPress={() => onPress(media)} style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}>
      <Poster media={media}>
        <Text style={[styles.badge, styles.kind]}>{isVideo ? "VIDEO" : "COMIC"}</Text>
        {media.video && (
          <Text style={[styles.badge, styles.duration]}>{formatDuration(media.video.durationMs / 1000)}</Text>
        )}
      </Poster>
      <Text numberOfLines={2} style={[styles.title, { color: colors.text }]}>
        {media.title}
      </Text>
    </Pressable>
  );
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
  title: { fontSize: 14, fontWeight: "500" },
});
