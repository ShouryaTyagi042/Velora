import { Image } from "expo-image";
import { useState } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { authHeaders } from "@/api/client";
import { MediaKind, thumbnailUrl, type Media } from "@/api/media";
import { motion, onMedia, radius, useAppTheme } from "@/theme";
import { Icon } from "./Icon";

type Props = { media: Media; style?: StyleProp<ViewStyle>; children?: React.ReactNode };

// Poster is a media item's picture in the shape of its kind: 16:9 for a video, 2:3 for a comic.
// Until there is a thumbnail (a video still being processed), or if it fails to load, it's a
// coloured placeholder with an icon in the same box, so nothing shifts when the image arrives;
// the image then fades in over it.
export function Poster({ media, style, children }: Props) {
  const { colors } = useAppTheme();
  const isVideo = media.kind === MediaKind.Video;
  const uri = thumbnailUrl(media);
  const [failed, setFailed] = useState<string | null>(null); // the uri that failed, so a new one retries

  return (
    <View
      style={[
        styles.poster,
        { aspectRatio: isVideo ? 16 / 9 : 2 / 3, backgroundColor: isVideo ? colors.posterVideo : colors.posterComic },
        style,
      ]}
    >
      {isVideo ? (
        <Icon sf="film" md="movie" size={28} color={onMedia.placeholderIcon} />
      ) : (
        <Icon sf="book.closed" md="menu_book" size={28} color={onMedia.placeholderIcon} />
      )}
      {uri && failed !== uri && (
        <Image
          source={{ uri, headers: authHeaders() }}
          style={StyleSheet.absoluteFill}
          contentFit="cover"
          transition={motion.fast}
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
  poster: { borderRadius: radius.md, alignItems: "center", justifyContent: "center", overflow: "hidden" },
});
