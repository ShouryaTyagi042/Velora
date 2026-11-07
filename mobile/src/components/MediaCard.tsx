import * as Haptics from "expo-haptics";
import { Pressable, StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { MediaKind, Section, type Media, type Progress } from "@/api/media";
import { formatDuration, formatResolution, formatViews } from "@/lib/format";
import { ComicPreview } from "@/preview/ComicPreview";
import { PreviewVideo } from "@/preview/PreviewVideo";
import { startPreview, useIsPreviewing } from "@/preview/store";
import { gradients, onMedia, radius, space, useAppTheme } from "@/theme";
import { Gradient } from "./Gradient";
import { Icon } from "./Icon";
import { Poster } from "./Poster";
import { Text } from "./Text";

type Props = {
  media: Media;
  onPress: (media: Media) => void;
  // rail: a card in a horizontal row (Home), title under the poster. wide: one card per row,
  // full width (Actors, Anime, Search), with the title over the bottom of the artwork and a
  // detail line under it.
  layout?: "rail" | "wide";
  showKind?: boolean; // a small Anime / Comic tag, where kinds mix (Search)
  style?: StyleProp<ViewStyle>;
};

// MediaCard is a poster and a title. A video shows its duration once known, anything started a
// thin Velora-blue progress bar along the poster's bottom edge.
//
// Long-press previews it, with a haptic tap: a video plays its short silent montage over the
// poster, a comic flips through its first pages. Once, then back to the poster; another
// long-press, leaving the screen or scrolling far away ends it sooner. Nothing happens on release.
export function MediaCard({ media, onPress, layout = "rail", showKind = false, style }: Props) {
  const { colors } = useAppTheme();
  const isVideo = media.kind === MediaKind.Video;
  const canPreview = isVideo ? media.video?.preview === true : (media.comic?.pageCount ?? 0) > 1;
  const previewing = useIsPreviewing(media.id);
  const done = media.progress && !media.progress.completed ? fraction(media.progress) : null;
  const wide = layout === "wide";
  const preview = () => {
    void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light); // the press was felt: it's intentional
    startPreview(media);
  };

  return (
    <Pressable
      onPress={() => onPress(media)}
      onLongPress={canPreview ? preview : undefined}
      delayLongPress={PREVIEW_DELAY}
      accessibilityRole="button"
      accessibilityLabel={describe(media, done)}
      accessibilityActions={canPreview ? [{ name: "longpress", label: "Preview" }] : undefined}
      onAccessibilityAction={(e) => e.nativeEvent.actionName === "longpress" && canPreview && preview()}
      style={({ pressed }) => [wide ? styles.wide : styles.rail, style, pressed && styles.pressed]}
    >
      <Poster media={media} style={wide && styles.wideArt}>
        {previewing && (isVideo ? <PreviewVideo id={media.id} /> : <ComicPreview comic={media} />)}
        {wide && !previewing && (
          <>
            <Gradient colors={gradients.posterBottom} style={styles.wideFade} />
            <Text variant="title" color={onMedia.text} numberOfLines={2} style={styles.wideTitle}>
              {media.title}
            </Text>
          </>
        )}
        {showKind && !previewing && <Kind media={media} />}
        {media.favourite && !previewing && (
          <View style={[styles.heart, { backgroundColor: onMedia.scrim }]} accessible={false}>
            <Icon sf="heart.fill" md="favorite" size={11} color={colors.primaryText} />
          </View>
        )}
        {media.video && !previewing && (
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
      {wide ? (
        <Text variant="caption" muted numberOfLines={1} style={styles.wideMeta}>
          {detailLine(media)}
        </Text>
      ) : (
        <Text variant="caption" numberOfLines={2} style={styles.railTitle}>
          {media.title}
        </Text>
      )}
    </Pressable>
  );
}

function Kind({ media }: { media: Media }) {
  const { colors } = useAppTheme();
  const label = media.kind === MediaKind.Comic ? "Comic" : media.section === Section.Anime ? "Anime" : "Video";
  return (
    <Text variant="caption" color={onMedia.text} style={[styles.badge, styles.kind, { backgroundColor: label === "Video" ? onMedia.scrim : colors.primary }]}>
      {label}
    </Text>
  );
}

// PREVIEW_DELAY is how long a press must last to start a preview, in ms.
const PREVIEW_DELAY = 350;

// fraction is how far through an item its progress is, 0 to 1.
function fraction(p: Progress): number {
  if (p.durationMs) return Math.min(1, (p.positionMs ?? 0) / p.durationMs);
  if (p.pageCount) return Math.min(1, ((p.page ?? 0) + 1) / p.pageCount);
  return 0;
}

// detailLine is what a wide card says under its artwork: who's in it or which show, and its size.
export function detailLine(m: Media): string {
  const parts: string[] = [];
  if (m.series) parts.push(m.series);
  else if (m.actors?.length) parts.push(m.actors.map((a) => a.name).join(", "));
  if (m.video) parts.push(formatResolution(m.video.width, m.video.height));
  if (m.comic) parts.push(`${m.comic.pageCount} pages`);
  if (m.kind === MediaKind.Video && !m.video) parts.push("Still processing");
  if (m.views > 0) parts.push(formatViews(m.views));
  if (m.tags?.length) parts.push(m.tags.slice(0, 3).map((t) => `#${t}`).join(" "));
  return parts.join("  ·  ");
}

function describe(m: Media, done: number | null): string {
  const parts = [m.title, m.kind === MediaKind.Video ? (m.section === Section.Anime ? "anime" : "video") : "comic"];
  if (m.video) parts.push(formatDuration(m.video.durationMs / 1000));
  if (done !== null) parts.push(`${Math.round(done * 100)}% ${m.kind === MediaKind.Video ? "watched" : "read"}`);
  if (m.favourite) parts.push("favourite");
  return parts.join(", ");
}

const styles = StyleSheet.create({
  rail: { gap: space.xs + 2 },
  railTitle: { paddingRight: space.xs },
  wide: { gap: space.sm },
  wideArt: { borderRadius: radius.md },
  wideFade: { top: "45%" },
  wideTitle: { position: "absolute", left: space.md, right: space.xxxl + space.lg, bottom: space.md },
  wideMeta: { paddingHorizontal: space.xs },
  pressed: { opacity: 0.7 },
  badge: {
    position: "absolute",
    fontWeight: "700",
    backgroundColor: onMedia.scrim,
    paddingHorizontal: space.xs + 1,
    paddingVertical: 1,
    borderRadius: radius.sm,
    overflow: "hidden",
  },
  kind: { top: space.sm, left: space.sm },
  heart: { position: "absolute", top: space.sm, right: space.sm, width: 22, height: 22, borderRadius: 11, alignItems: "center", justifyContent: "center" },
  duration: { bottom: space.sm + 2, right: space.sm, fontVariant: ["tabular-nums"] },
  track: { position: "absolute", left: 0, right: 0, bottom: 0, height: 3 },
  bar: { height: "100%" },
});
