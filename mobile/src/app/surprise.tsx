import * as Haptics from "expo-haptics";
import { router, useLocalSearchParams } from "expo-router";
import { Image } from "expo-image";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ScrollView, StyleSheet, View } from "react-native";

import { authHeaders } from "@/api/client";
import { MediaKind, pageUrl, randomMedia, Section, type Media } from "@/api/media";
import { Button } from "@/components/Button";
import { Chip } from "@/components/Chip";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { detailLine } from "@/components/MediaCard";
import { Poster } from "@/components/Poster";
import { Skeleton } from "@/components/Skeleton";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { formatDuration } from "@/lib/format";
import { latePages } from "@/lib/pages";
import { VideoTeaser } from "@/preview/LoopingClip";
import { resumeAt, resumePage } from "@/progress/resume";
import { motion, onMedia, radius, space, useAppTheme } from "@/theme";

type SurpriseKind = "any" | "film" | "anime" | "comic";

const KINDS: { kind: SurpriseKind; label: string }[] = [
  { kind: "any", label: "Anything" },
  { kind: "film", label: "Film" },
  { kind: "anime", label: "Anime" },
  { kind: "comic", label: "Comic" },
];

// pick is one item at random of the chosen kind. "Anything" leans to what's processed: a video
// still waiting for its still and duration is a poor first impression.
async function pick(kind: SurpriseKind, signal: AbortSignal): Promise<Media | null> {
  const opts =
    kind === "film"
      ? { kind: MediaKind.Video, section: Section.Movies }
      : kind === "anime"
        ? { kind: MediaKind.Video, section: Section.Anime }
        : kind === "comic"
          ? { kind: MediaKind.Comic }
          : {};
  const items = await randomMedia(5, { ...opts, signal });
  // Prefer what can preview itself on the sheet: a video with a clip, a comic with pages to flip.
  return (
    items.find((m) => (m.kind === MediaKind.Comic ? (m.comic?.pageCount ?? 0) > 2 : m.video?.short || m.video?.preview)) ??
    items.find((m) => m.kind === MediaKind.Comic || m.video) ??
    items[0] ??
    null
  );
}

// Surprise me: a sheet with one thing picked at random from your library, of the kind you chose.
// Play it, look closer, or shuffle again. Each shuffle is a new pick from the server.
export default function Surprise() {
  const params = useLocalSearchParams<{ kind?: string }>();
  const [kind, setKind] = useState<SurpriseKind>(KINDS.some((k) => k.kind === params.kind) ? (params.kind as SurpriseKind) : "any");
  const fetcher = useCallback((signal: AbortSignal) => pick(kind, signal), [kind]);
  const { state, refetch } = useAsync(fetcher);
  const { colors } = useAppTheme();

  const shuffle = () => {
    void Haptics.selectionAsync();
    refetch();
  };

  return (
    <ScrollView contentContainerStyle={styles.content} style={{ backgroundColor: colors.background }}>
      <View style={styles.kinds}>
        {KINDS.map((k) => (
          <Chip key={k.kind} label={k.label} selected={kind === k.kind} onPress={() => setKind(k.kind)} />
        ))}
      </View>
      {state.status === "loading" ? (
        <View style={styles.reveal} accessibilityLabel="Picking">
          <Skeleton aspectRatio={16 / 9} />
          <Skeleton width="70%" height={30} />
          <Skeleton height={44} />
        </View>
      ) : state.status === "error" ? (
        <ErrorView error={state.error} onRetry={refetch} />
      ) : state.data === null ? (
        <EmptyView title="Nothing to pick from" detail="There's nothing of this kind in your library yet. Try another kind." />
      ) : (
        // Keyed by the pick: a new pick is a new preview, from its start.
        <Reveal key={state.data.id} media={state.data} onShuffle={shuffle} />
      )}
    </ScrollView>
  );
}

function Reveal({ media, onShuffle }: { media: Media; onShuffle: () => void }) {
  const isVideo = media.kind === MediaKind.Video;
  const go = (pathname: "/player/[id]" | "/reader/[id]" | "/media/[id]") => {
    router.back(); // close the sheet, then open it from where you were
    router.push({ pathname, params: { id: media.id } });
  };
  const action = isVideo ? (resumeAt(media) > 0 ? "Resume" : "Play") : resumePage(media) > 0 ? "Continue reading" : "Read";
  const meta = [detailLine(media), media.video && formatDuration(media.video.durationMs / 1000)].filter(Boolean).join("  ·  ");

  return (
    <View style={styles.reveal}>
      <Poster media={media} style={isVideo ? styles.still : styles.comic}>
        {isVideo ? <VideoTeaser media={media} /> : <PageFlip comic={media} />}
      </Poster>
      <View style={styles.text}>
        <Text variant="display" accessibilityRole="header">
          {media.title}
        </Text>
        {meta ? (
          <Text variant="label" muted>
            {meta}
          </Text>
        ) : null}
      </View>
      <Button
        label={action}
        icon={isVideo ? { sf: "play.fill", md: "play_arrow" } : { sf: "book.fill", md: "menu_book" }}
        variant="play"
        haptic
        onPress={() => go(isVideo ? "/player/[id]" : "/reader/[id]")}
      />
      <View style={styles.row}>
        <Button label="Shuffle again" icon={{ sf: "shuffle", md: "shuffle" }} variant="secondary" onPress={onShuffle} style={styles.half} />
        {isVideo && <Button label="Details" icon={{ sf: "info.circle", md: "info" }} variant="secondary" onPress={() => go("/media/[id]")} style={styles.half} />}
      </View>
    </View>
  );
}

// PageFlip turns a comic's late pages (from past 60%) over its cover, one after another, round
// and round, while the sheet is open: the comic's own preview.
const FLIP_PAGES = 8;
const FLIP_MS = 1400;

function PageFlip({ comic }: { comic: Media }) {
  const urls = useMemo(() => latePages(comic.comic?.pageCount ?? 0, FLIP_PAGES).map((p) => pageUrl(comic, p)), [comic]);
  const [n, setN] = useState(0);

  useEffect(() => {
    void Image.prefetch(urls, { cachePolicy: "disk", headers: authHeaders() });
  }, [urls]);
  useEffect(() => {
    if (urls.length < 2) return;
    const t = setTimeout(() => setN((i) => (i + 1) % urls.length), FLIP_MS);
    return () => clearTimeout(t);
  }, [n, urls.length]);

  if (urls.length === 0) return null;
  return (
    <View style={[StyleSheet.absoluteFill, { backgroundColor: onMedia.backdrop }]} pointerEvents="none">
      <Image source={{ uri: urls[n], headers: authHeaders() }} style={StyleSheet.absoluteFill} contentFit="contain" transition={motion.fast} accessible={false} />
    </View>
  );
}

const styles = StyleSheet.create({
  content: { padding: space.lg, paddingTop: space.xl, gap: space.xl, flexGrow: 1 },
  kinds: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  reveal: { gap: space.lg },
  still: { borderRadius: radius.md },
  comic: { width: "56%", alignSelf: "center", borderRadius: radius.md },
  text: { gap: space.xs },
  row: { flexDirection: "row", gap: space.sm },
  half: { flex: 1 },
});
