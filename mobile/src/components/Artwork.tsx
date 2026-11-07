import { Image } from "expo-image";
import { useState } from "react";
import { StyleSheet, View, type ColorValue, type StyleProp, type ViewStyle } from "react-native";

import { authHeaders } from "@/api/client";
import { motion, onMedia, radius } from "@/theme";
import { Icon } from "./Icon";

export type ArtworkIcon = "film" | "book" | "person" | "tv";

const ICONS = {
  film: { sf: "film", md: "movie" },
  book: { sf: "book.closed", md: "menu_book" },
  person: { sf: "person.fill", md: "person" },
  tv: { sf: "sparkles.tv", md: "live_tv" },
} as const;

type Props = {
  uri: string | null; // null: nothing to load yet (a video still being processed)
  aspectRatio: number;
  placeholder: { icon: ArtworkIcon; color: ColorValue };
  style?: StyleProp<ViewStyle>;
  children?: React.ReactNode;
};

// Artwork is an image from the server in a fixed shape: a coloured placeholder with an icon in the
// same box first, so nothing shifts when the image arrives, and stays if it fails to load. The
// image fades in over it. Every request carries the session header.
export function Artwork({ uri, aspectRatio, placeholder, style, children }: Props) {
  const [failed, setFailed] = useState<string | null>(null); // the uri that failed, so a new one retries
  const icon = ICONS[placeholder.icon];
  return (
    <View style={[styles.box, { aspectRatio, backgroundColor: placeholder.color }, style]}>
      <Icon sf={icon.sf} md={icon.md} size={28} color={onMedia.placeholderIcon} />
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
  box: { borderRadius: radius.sm, alignItems: "center", justifyContent: "center", overflow: "hidden" },
});
