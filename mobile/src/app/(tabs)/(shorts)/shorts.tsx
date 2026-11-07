import { useEventListener } from "expo";
import { Image } from "expo-image";
import { router, useIsFocused } from "expo-router";
import { useVideoPlayer, VideoView, type VideoPlayer } from "expo-video";
import { useCallback, useEffect, useRef, useState } from "react";
import { FlatList, Pressable, StyleSheet, useWindowDimensions, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { authHeaders } from "@/api/client";
import { MediaKind, randomMedia, Section, streamUrl, thumbnailUrl, type Media, type Page } from "@/api/media";
import { Button } from "@/components/Button";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Gradient } from "@/components/Gradient";
import { Icon } from "@/components/Icon";
import { Loading } from "@/components/Loading";
import { Text } from "@/components/Text";
import { usePages } from "@/hooks/usePages";
import { usePlayerStatus } from "@/hooks/usePlayerStatus";
import { detailLine } from "@/components/MediaCard";
import { resumeAt } from "@/progress/resume";
import { gradients, onMedia, palette, radius, space, touch, typography, useAppTheme } from "@/theme";

// How many of each shelf one batch asks for. The feed alternates them, so a library with ten
// films and three hundred episodes still shows both.
const BATCH = 8;
// Where in a video a short starts: somewhere in its middle, past openings and cold opens, short
// of the ending. A fraction of the duration.
const START_FROM = 0.12;
const START_TO = 0.65;

type Short = { media: Media; key: string; startAt: number };

let keySeq = 0;

// nextBatch is a fresh shuffle of films and anime, interleaved. Only videos with metadata: a
// short needs a duration to start in the middle, and a thumbnail behind it.
async function nextBatch(signal?: AbortSignal): Promise<Short[]> {
  const [films, anime] = await Promise.all([
    randomMedia(BATCH, { kind: MediaKind.Video, section: Section.Movies, signal }),
    randomMedia(BATCH, { kind: MediaKind.Video, section: Section.Anime, signal }),
  ]);
  const ready = (ms: Media[]) => ms.filter((m) => m.video && m.video.durationMs > 0);
  const a = ready(films);
  const b = ready(anime);
  const out: Short[] = [];
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    for (const m of [a[i], b[i]]) {
      if (!m) continue;
      const span = START_FROM + Math.random() * (START_TO - START_FROM);
      out.push({ media: m, key: `${m.id}:${++keySeq}`, startAt: ((m.video?.durationMs ?? 0) / 1000) * span });
    }
  }
  return out;
}

// Muted or not is the feed's setting, kept while the app runs: swiping on shouldn't unmute.
let mutedSetting = false;

// Shorts: a vertical feed of moments from your own library, films and anime in turn, each
// starting somewhere in the middle. Swipe up for the next. Tap to pause; Watch opens the full
// thing (resuming where you left off, if you had started it); the speaker toggles sound.
// One native player serves the whole feed: the visible short gets it, the others show a still.
export default function Shorts() {
  const { height: windowHeight } = useWindowDimensions();
  const [height, setHeight] = useState(windowHeight);
  const [active, setActive] = useState(0);
  const list = useRef<FlatList<Short>>(null);
  const focused = useIsFocused();
  const focusedNow = useRef(focused);

  // The feed never ends: every "page" is a new shuffle, so there is always a next one.
  const { state, refetch, loadMore } = usePages(fetchShorts);
  const items = state.status === "ok" ? state.data : null;

  const player = useVideoPlayer(null, (p) => {
    p.loop = true;
    p.muted = mutedSetting;
    p.timeUpdateEventInterval = 0.25;
  });

  // The active short gets the player: its stream, from its starting point.
  const current = items?.[active];
  useEffect(() => {
    if (!current) return;
    let live = true;
    void player
      .replaceAsync({ uri: streamUrl(current.media.id), headers: authHeaders(), metadata: { title: current.media.title } })
      .then(() => {
        if (!live) return;
        seekTo(player, current.startAt);
        if (focusedNow.current) player.play();
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [current, player]);

  // Leaving the tab pauses; coming back plays again, without reloading the stream.
  useEffect(() => {
    focusedNow.current = focused;
    if (!focused) player.pause();
    else if (player.status === "readyToPlay") player.play();
  }, [focused, player]);

  const onViewable = useCallback(({ viewableItems }: { viewableItems: { index?: number | null; isViewable: boolean }[] }) => {
    const first = viewableItems.find((v) => v.isViewable);
    if (first?.index != null) setActive(first.index);
  }, []);

  const shuffle = () => {
    list.current?.scrollToOffset({ offset: 0, animated: false });
    setActive(0);
    refetch();
  };

  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  if (!items) return <Loading />;
  if (items.length === 0) {
    return (
      <EmptyView
        title="Nothing to shuffle yet"
        detail="Shorts play moments from your videos and anime once the server has processed them. Add some to movies/ or anime/ and rescan."
        action={{ label: "Try again", onPress: refetch }}
      />
    );
  }

  return (
    <View style={styles.screen} onLayout={(e) => setHeight(e.nativeEvent.layout.height)}>
      <FlatList
        ref={list}
        data={items}
        keyExtractor={(s) => s.key}
        renderItem={({ item, index }) => <ShortPage short={item} height={height} player={index === active ? player : null} />}
        pagingEnabled
        showsVerticalScrollIndicator={false}
        contentInsetAdjustmentBehavior="never"
        decelerationRate="fast"
        getItemLayout={(_, index) => ({ length: height, offset: height * index, index })}
        onViewableItemsChanged={onViewable}
        viewabilityConfig={VIEWABILITY}
        // Near the end of what's loaded, shuffle in more.
        onEndReached={loadMore}
        onEndReachedThreshold={2}
        windowSize={3}
        initialNumToRender={2}
        maxToRenderPerBatch={2}
        removeClippedSubviews
      />
      <ShortsHeader onShuffle={shuffle} />
    </View>
  );
}

const VIEWABILITY = { itemVisiblePercentThreshold: 60 };

// fetchShorts is usePages' fetch: a new shuffle for every page, with a cursor that never runs out.
const fetchShorts = async (_after: string | undefined, signal: AbortSignal): Promise<Page<Short>> => ({
  items: await nextBatch(signal),
  nextCursor: "more",
});

// The player is a native object: setting its properties is a command, not a React mutation. These
// helpers keep that out of components, where the compiler (rightly) treats props as immutable.
function seekTo(p: VideoPlayer, seconds: number) {
  p.currentTime = seconds;
}
function setPlayerMuted(p: VideoPlayer, muted: boolean) {
  p.muted = muted;
}

function ShortsHeader({ onShuffle }: { onShuffle: () => void }) {
  const insets = useSafeAreaInsets();
  return (
    <View style={[styles.header, { paddingTop: insets.top + space.xs }]} pointerEvents="box-none">
      <Gradient colors={gradients.top} />
      <Text style={styles.headerTitle} color={onMedia.text} accessibilityRole="header">
        Shorts
      </Text>
      <Pressable onPress={onShuffle} hitSlop={8} accessibilityRole="button" accessibilityLabel="Shuffle a new feed" style={styles.round}>
        <Icon sf="shuffle" md="shuffle" size={20} color={onMedia.text} />
      </Pressable>
    </View>
  );
}

type PageProps = { short: Short; height: number; player: VideoPlayer | null };

// ShortPage is one screenful: the still, blurred, filling the screen behind the video at its own
// shape, and the title and actions over the bottom. Only the active page has the player.
function ShortPage({ short, height, player }: PageProps) {
  const { width } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const { media } = short;
  const thumb = thumbnailUrl(media);
  const videoHeight = width / Math.max(0.5, (media.video?.width ?? 16) / (media.video?.height || 9));
  const anime = media.section === Section.Anime;

  return (
    <View style={[styles.page, { height }]}>
      {thumb && (
        <Image source={{ uri: thumb, headers: authHeaders() }} style={StyleSheet.absoluteFill} contentFit="cover" blurRadius={40} accessible={false} />
      )}
      <View style={[StyleSheet.absoluteFill, styles.dim]} />
      <View style={[styles.stage, { height: videoHeight, marginTop: -videoHeight * 0.15 }]}>
        {thumb && <Image source={{ uri: thumb, headers: authHeaders() }} style={StyleSheet.absoluteFill} contentFit="cover" accessible={false} />}
        {player && <ActiveVideo player={player} />}
      </View>

      <Gradient colors={gradients.posterBottom} style={styles.bottomFade} />
      {/* Clear of the floating tab bar: its height plus the home indicator. */}
      <View style={[styles.info, { paddingBottom: insets.bottom + 72 }]}>
        <Text variant="caption" color={onMedia.text} style={[styles.tag, anime && styles.tagAnime]}>
          {anime ? "Anime" : "Film"}
        </Text>
        <Text style={styles.title} color={onMedia.text} numberOfLines={2}>
          {media.title}
        </Text>
        <Text variant="label" color={onMedia.muted} numberOfLines={1}>
          {detailLine(media)}
        </Text>
        <View style={styles.actions}>
          <Button
            label={resumeAt(media) > 0 ? "Resume" : "Watch"}
            icon={{ sf: "play.fill", md: "play_arrow" }}
            variant="play"
            haptic
            onPress={() => router.push({ pathname: "/player/[id]", params: { id: media.id } })}
            style={styles.watch}
          />
          <Button
            label="Details"
            icon={{ sf: "info.circle", md: "info" }}
            variant="secondary"
            onPress={() => router.push({ pathname: "/media/[id]", params: { id: media.id } })}
            style={styles.watch}
          />
        </View>
      </View>
      {player && <Scrubber player={player} bottom={insets.bottom + 60} />}
    </View>
  );
}

// ActiveVideo is the shared player in the active page, with tap-to-pause and the sound toggle.
function ActiveVideo({ player }: { player: VideoPlayer }) {
  const s = usePlayerStatus(player);
  const [muted, setMuted] = useState(player.muted);
  const toggleMute = () => {
    mutedSetting = !muted;
    setPlayerMuted(player, mutedSetting);
    setMuted(mutedSetting);
  };
  useEventListener(player, "mutedChange", ({ muted: m }) => setMuted(m));

  return (
    <>
      <VideoView player={player} style={StyleSheet.absoluteFill} contentFit="cover" nativeControls={false} allowsPictureInPicture={false} />
      <Pressable
        style={[StyleSheet.absoluteFill, styles.center]}
        onPress={() => (player.playing ? player.pause() : player.play())}
        accessibilityRole="button"
        accessibilityLabel={s.playing ? "Pause" : "Play"}
      >
        {!s.playing && s.status === "readyToPlay" && (
          <View style={styles.bigGlyph}>
            <Icon sf="play.fill" md="play_arrow" size={30} color={onMedia.text} />
          </View>
        )}
      </Pressable>
      <Pressable
        onPress={toggleMute}
        hitSlop={8}
        accessibilityRole="button"
        accessibilityLabel={muted ? "Turn sound on" : "Turn sound off"}
        style={[styles.round, styles.mute]}
      >
        <Icon sf={muted ? "speaker.slash.fill" : "speaker.wave.2.fill"} md={muted ? "volume_off" : "volume_up"} size={16} color={onMedia.text} />
      </Pressable>
    </>
  );
}

// Scrubber is a hairline of how far into the video this moment is: not a control, a sense of place.
function Scrubber({ player, bottom }: { player: VideoPlayer; bottom: number }) {
  const { colors } = useAppTheme();
  const s = usePlayerStatus(player);
  const done = s.duration > 0 ? Math.min(1, s.currentTime / s.duration) : 0;
  return (
    <View style={[styles.scrub, { bottom, backgroundColor: onMedia.track }]} pointerEvents="none">
      <View style={{ width: `${done * 100}%`, height: "100%", backgroundColor: colors.primary }} />
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: onMedia.backdrop },
  header: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: space.lg,
    paddingBottom: space.lg,
  },
  headerTitle: { ...typography.display },
  page: { justifyContent: "center", overflow: "hidden" },
  dim: { backgroundColor: onMedia.scrim },
  stage: { width: "100%", backgroundColor: onMedia.backdrop },
  center: { alignItems: "center", justifyContent: "center" },
  bigGlyph: { width: 64, height: 64, borderRadius: 32, alignItems: "center", justifyContent: "center", backgroundColor: onMedia.scrim },
  round: {
    width: touch.min - 6,
    height: touch.min - 6,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: onMedia.scrim,
  },
  mute: { position: "absolute", right: space.md, bottom: space.md },
  bottomFade: { top: "55%" },
  info: { position: "absolute", left: 0, right: 0, bottom: 0, paddingHorizontal: space.lg, gap: space.sm },
  tag: {
    alignSelf: "flex-start",
    fontWeight: "700",
    paddingHorizontal: space.sm,
    paddingVertical: 2,
    borderRadius: radius.sm,
    overflow: "hidden",
    backgroundColor: onMedia.scrim,
  },
  tagAnime: { backgroundColor: palette.dark.primary },
  title: { ...typography.display },
  actions: { flexDirection: "row", gap: space.sm, marginTop: space.sm },
  watch: { flex: 1 },
  scrub: { position: "absolute", left: space.lg, right: space.lg, height: 2, borderRadius: 1, overflow: "hidden" },
});
