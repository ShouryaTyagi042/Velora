import { Pressable, StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { coverUrl, type Cover } from "@/api/media";
import { gradients, onMedia, radius, space, typography, useAppTheme } from "@/theme";
import { Artwork, type ArtworkIcon } from "./Artwork";
import { Gradient } from "./Gradient";
import { Text } from "./Text";

type Props = {
  name: string;
  count: string; // "12 videos", "24 episodes"
  cover?: Cover;
  icon: ArtworkIcon;
  onPress: () => void;
  // banner: full width, one per row (the Actors and Anime tabs). tile: a fixed-width card in a rail.
  layout?: "banner" | "tile";
  style?: StyleProp<ViewStyle>;
};

// CoverCard stands for a group of videos (an actor, an anime series) by one of its thumbnails,
// with the name set large over the bottom of it, the way a streaming app shows a show.
export function CoverCard({ name, count, cover, icon, onPress, layout = "banner", style }: Props) {
  const { colors } = useAppTheme();
  const banner = layout === "banner";
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={`${name}, ${count}`}
      style={({ pressed }) => [style, pressed && styles.pressed]}
    >
      <Artwork
        uri={cover ? coverUrl(cover) : null}
        aspectRatio={banner ? 16 / 8 : 16 / 10}
        placeholder={{ icon, color: colors.posterVideo }}
        style={banner && styles.bannerArt}
      >
        <Gradient colors={gradients.posterBottom} style={styles.fade} />
        <View style={[styles.caption, banner ? styles.bannerCaption : styles.tileCaption]}>
          <Text style={banner ? styles.bannerName : styles.tileName} color={onMedia.text} numberOfLines={2}>
            {name}
          </Text>
          <Text variant="caption" color={onMedia.muted}>
            {count}
          </Text>
        </View>
        {/* The brand's thread: a short blue rule under every group's name. */}
        <View style={[styles.rule, banner ? styles.bannerRule : styles.tileRule, { backgroundColor: colors.primary }]} />
      </Artwork>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  pressed: { opacity: 0.75 },
  bannerArt: { borderRadius: radius.md },
  fade: { top: "30%" },
  caption: { position: "absolute", left: 0, right: 0, gap: 2 },
  bannerCaption: { paddingHorizontal: space.lg, bottom: space.lg },
  tileCaption: { paddingHorizontal: space.sm + 2, bottom: space.sm + 2 },
  bannerName: { ...typography.display },
  tileName: { ...typography.label, fontWeight: "800" },
  rule: { position: "absolute", left: 0, top: 0, width: 3 },
  bannerRule: { height: 28, top: space.lg },
  tileRule: { height: 18, top: space.sm + 2 },
});
