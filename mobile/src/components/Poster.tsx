import type { StyleProp, ViewStyle } from "react-native";

import { MediaKind, thumbnailUrl, type Media } from "@/api/media";
import { useAppTheme } from "@/theme";
import { Artwork } from "./Artwork";

type Props = { media: Media; aspectRatio?: number; style?: StyleProp<ViewStyle>; children?: React.ReactNode };

// Poster is a media item's picture in the shape of its kind: 16:9 for a video, 2:3 for a comic,
// unless the layout asks for another crop (the billboard).
export function Poster({ media, aspectRatio, style, children }: Props) {
  const { colors } = useAppTheme();
  const isVideo = media.kind === MediaKind.Video;
  return (
    <Artwork
      uri={thumbnailUrl(media)}
      aspectRatio={aspectRatio ?? (isVideo ? 16 / 9 : 2 / 3)}
      placeholder={isVideo ? { icon: "film", color: colors.posterVideo } : { icon: "book", color: colors.posterComic }}
      style={style}
    >
      {children}
    </Artwork>
  );
}
