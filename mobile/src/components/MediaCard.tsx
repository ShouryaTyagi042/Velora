import { useTheme } from "expo-router";
import { Pressable, StyleSheet, Text, View, type ViewStyle } from "react-native";

import { MediaKind, type Media } from "@/api/media";

type Props = { media: Media; onPress: (media: Media) => void; style?: ViewStyle };

// MediaCard is a poster and a title. The poster is a placeholder until phase 9's thumbnails;
// its shape already matches the kind: 16:9 for a video, 2:3 for a comic page.
export function MediaCard({ media, onPress, style }: Props) {
  const { colors } = useTheme();
  const isVideo = media.kind === MediaKind.Video;
  return (
    <Pressable onPress={() => onPress(media)} style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}>
      <View
        style={[
          styles.poster,
          { aspectRatio: isVideo ? 16 / 9 : 2 / 3, backgroundColor: isVideo ? "#2b3a55" : "#553a2b" },
        ]}
      >
        <Text style={styles.glyph}>{isVideo ? "▶" : "▤"}</Text>
        <Text style={styles.badge}>{isVideo ? "VIDEO" : "COMIC"}</Text>
      </View>
      <Text numberOfLines={2} style={[styles.title, { color: colors.text }]}>
        {media.title}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { gap: 6 },
  pressed: { opacity: 0.6 },
  poster: { borderRadius: 8, alignItems: "center", justifyContent: "center" },
  glyph: { color: "#ffffffcc", fontSize: 28 },
  badge: {
    position: "absolute",
    top: 6,
    left: 6,
    color: "#fff",
    fontSize: 10,
    fontWeight: "700",
    backgroundColor: "#00000066",
    paddingHorizontal: 5,
    paddingVertical: 2,
    borderRadius: 4,
    overflow: "hidden",
  },
  title: { fontSize: 14, fontWeight: "500" },
});
