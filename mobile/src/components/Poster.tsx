import { Image } from "expo-image";
import { useState } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";

import { MediaKind, thumbnailUrl, type Media } from "@/api/media";

type Props = { media: Media; style?: StyleProp<ViewStyle>; children?: React.ReactNode };

// Poster is a media item's picture in the shape of its kind: 16:9 for a video, 2:3 for a comic.
// Until there is a thumbnail (a video still being processed), or if it fails to load, it's a
// coloured placeholder in the same box, so nothing shifts when the image arrives; the image
// then fades in over it.
export function Poster({ media, style, children }: Props) {
  const isVideo = media.kind === MediaKind.Video;
  const uri = thumbnailUrl(media);
  const [failed, setFailed] = useState<string | null>(null); // the uri that failed, so a new one retries

  return (
    <View
      style={[styles.poster, { aspectRatio: isVideo ? 16 / 9 : 2 / 3, backgroundColor: isVideo ? "#2b3a55" : "#553a2b" }, style]}
    >
      <Text style={styles.glyph}>{isVideo ? "▶" : "▤"}</Text>
      {uri && failed !== uri && (
        <Image
          source={{ uri }}
          style={StyleSheet.absoluteFill}
          contentFit="cover"
          transition={150}
          recyclingKey={uri}
          onError={() => setFailed(uri)}
          accessible={false}
        />
      )}
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  poster: { borderRadius: 8, alignItems: "center", justifyContent: "center", overflow: "hidden" },
  glyph: { color: "#ffffffcc", fontSize: 28 },
});
