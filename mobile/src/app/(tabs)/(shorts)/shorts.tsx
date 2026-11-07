import { Image } from "expo-image";
import { router, useIsFocused } from "expo-router";
import { useVideoPlayer, VideoView, type VideoPlayer } from "expo-video";
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { FlatList, Pressable, ScrollView, StyleSheet, useWindowDimensions, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { authHeaders } from "@/api/client";
import { MediaKind, pageUrl, previewUrl, randomMedia, Section, shortUrl, thumbnailUrl, type Media, type Page } from "@/api/media";
import { Button } from "@/components/Button";
import { Chip } from "@/components/Chip";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Gradient } from "@/components/Gradient";
import { Icon } from "@/components/Icon";
import { Loading } from "@/components/Loading";
import { detailLine } from "@/components/MediaCard";
import { Text } from "@/components/Text";
import { usePages } from "@/hooks/usePages";
import { usePlayerStatus } from "@/hooks/usePlayerStatus";
import { latePages } from "@/lib/pages";
import { resumeAt, resumePage } from "@/progress/resume";
import { gradients, onMedia, palette, radius, space, touch, typography, useAppTheme } from "@/theme";

// What the feed shuffles: everything, or one shelf.
type Shelf = "mix" | "videos" | "anime" | "comics";
const SHELVES: { shelf: Shelf; label: string }[] = [
  { shelf: "mix", label: "Mix" },
  { shelf: "videos", label: "Videos" },
  { shelf: "anime", label: "Anime" },
  { shelf: "comics", label: "Comics" },
];

// How many of each shelf one batch asks for. Mix takes turns between them, so a library with ten
// films and three hundred episodes still shows both.
const BATCH = 8;

// A comic's short: this many pages from past 60% of it, each up this long, then round again.
const COMIC_SHORT_PAGES = 10;
const COMIC_PAGE_MS = 3500;
// A comic needs at least this many pages to make a short worth swiping to.
const COMIC_MIN_PAGES = 6;

// A video's short plays a clip the server cut from it, never the video itself: its Shorts clip
// (eight 8 s moments, with sound) once made, and until then its long-press preview (four 3 s
// moments, silent, smaller), so the feed works while the server is still making clips. A comic's
// short flips through pages from its last 40%, like a story.
type Short =
  | { kind: "video"; media: Media; key: string; uri: string }
  | { kind: "comic"; media: Media; key: string; pages: string[] };

let keySeq = 0;

function toShort(m: Media): Short | null {
  const key = `${m.id}:${++keySeq}`;
  if (m.kind === MediaKind.Comic) {
    const count = m.comic?.pageCount ?? 0;
    if (count < COMIC_MIN_PAGES) return null;
    return { kind: "comic", media: m, key, pages: latePages(count, COMIC_SHORT_PAGES).map((p) => pageUrl(m, p)) };
  }
  const uri = m.video?.short ? shortUrl(m) : m.video?.preview ? previewUrl(m) : null;
  return uri ? { kind: "video", media: m, key, uri } : null;
}

async function comicBatch(n: number, signal: AbortSignal): Promise<Short[]> {
  const comics = await randomMedia(n, { kind: MediaKind.Comic, signal });
  return comics.map(toShort).filter((s): s is Short => s !== null);
}

// shelfBatch is up to n random videos of one shelf that have a clip, Shorts clips first.
async function shelfBatch(section: Section, n: number, signal: AbortSignal): Promise<Short[]> {
  const shorts = await randomMedia(n, { kind: MediaKind.Video, section, clip: "short", signal });
  const picked = shorts.length > 0 ? shorts : await randomMedia(n, { kind: MediaKind.Video, section, clip: "preview", signal });
  return picked.map(toShort).filter((s): s is Short => s !== null);
}

async function nextBatch(shelf: Shelf, signal: AbortSignal): Promise<Short[]> {
  if (shelf === "videos") return shelfBatch(Section.Movies, BATCH * 2, signal);
  if (shelf === "anime") return shelfBatch(Section.Anime, BATCH * 2, signal);
  if (shelf === "comics") return comicBatch(BATCH * 2, signal);
  const lists = await Promise.all([shelfBatch(Section.Movies, BATCH, signal), shelfBatch(Section.Anime, BATCH, signal), comicBatch(BATCH, signal)]);
  const out: Short[] = [];
  for (let i = 0; i < Math.max(...lists.map((l) => l.length)); i++) {
    for (const l of lists) if (l[i]) out.push(l[i]);
  }
  return out;
}

// Sound on or off is the feed's setting, kept while the app runs and shared by every page's
// player: swiping on shouldn't unmute.
let muted = false;
const muteListeners = new Set<() => void>();
function setMuted(m: boolean) {
  muted = m;
  muteListeners.forEach((l) => l());
}
function useMuted(): boolean {
  return useSyncExternalStore(
    (l) => {
      muteListeners.add(l);
      return () => {
        muteListeners.delete(l);
      };
    },
    () => muted,
  );
}

// The player is a native object: setting its properties is a command, not a React mutation. These
// helpers keep that out of components, where the compiler (rightly) treats props as immutable.
function applyMuted(p: VideoPlayer, m: boolean) {
  p.muted = m;
}
function rewind(p: VideoPlayer) {
  p.currentTime = 0;
}

// Shorts: a vertical feed of clips from your own library, films and anime in turn (or one of
// them), each a montage of moments from the video. Swipe up for the next. Tap to pause; Watch
// opens the full thing; the speaker toggles sound.
//
// The page you're on and the ones either side each have their own player, already loaded: a
// swipe lands on a clip that is ready, and a page shows its still until its own first frame is
// drawn, so the previous clip's last frame never shows on the next page.
export default function Shorts() {
  const { height: windowHeight } = useWindowDimensions();
  const [height, setHeight] = useState(windowHeight);
  const [shelf, setShelf] = useState<Shelf>("mix");
  const [active, setActive] = useState(0);
  const [paused, setPaused] = useState(false);
  const list = useRef<FlatList<Short>>(null);
  const focused = useIsFocused();

  // The feed never ends: every "page" is a new shuffle, so there is always a next one.
  const fetchShorts = useCallback(
    async (_after: string | undefined, signal: AbortSignal): Promise<Page<Short>> => ({ items: await nextBatch(shelf, signal), nextCursor: "more" }),
    [shelf],
  );
  const { state, refetch, loadMore } = usePages(fetchShorts);
  const items = state.status === "ok" ? state.data : null;

  const onViewable = useCallback(({ viewableItems }: { viewableItems: { index?: number | null; isViewable: boolean }[] }) => {
    const first = viewableItems.find((v) => v.isViewable);
    if (first?.index != null) {
      setActive(first.index);
      setPaused(false); // a new clip always starts playing
    }
  }, []);

  const restart = () => {
    list.current?.scrollToOffset({ offset: 0, animated: false });
    setActive(0);
    setPaused(false);
  };
  const shuffle = () => {
    restart();
    refetch();
  };
  const pick = (s: Shelf) => {
    if (s === shelf) return;
    restart();
    setShelf(s); // a new fetchShorts: usePages starts the feed again
  };

  let body: React.ReactElement;
  if (state.status === "error") {
    body = <ErrorView error={state.error} onRetry={refetch} />;
  } else if (!items) {
    body = <Loading />;
  } else if (items.length === 0) {
    body = (
      <EmptyView
        title="No clips yet"
        detail={
          shelf === "comics"
            ? `Comics play here once you have some with at least ${COMIC_MIN_PAGES} pages.`
            : "Shorts play clips the server cuts from your videos after it has processed them. Leave velora-server running for a while, then try again."
        }
        action={{ label: "Try again", onPress: refetch }}
      />
    );
  } else {
    body = (
      <FlatList
        ref={list}
        data={items}
        keyExtractor={(s) => s.key}
        renderItem={({ item, index }) => (
          <ShortPage
            short={item}
            height={height}
            near={Math.abs(index - active) <= 1}
            playing={index === active && focused && !paused}
            onTogglePause={() => setPaused((p) => !p)}
          />
        )}
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
        // Must stay off: a clipped page would lose its loaded player and flash on the way back.
        removeClippedSubviews={false}
        // Re-render pages when the active one or the pause changes (FlatList memoises rows).
        extraData={`${active}:${paused}:${focused}`}
      />
    );
  }

  return (
    <View style={styles.screen} onLayout={(e) => setHeight(e.nativeEvent.layout.height)}>
      {body}
      <ShortsHeader shelf={shelf} onPick={pick} onShuffle={shuffle} />
    </View>
  );
}

const VIEWABILITY = { itemVisiblePercentThreshold: 60 };

function ShortsHeader({ shelf, onPick, onShuffle }: { shelf: Shelf; onPick: (s: Shelf) => void; onShuffle: () => void }) {
  const insets = useSafeAreaInsets();
  return (
    <View style={[styles.header, { paddingTop: insets.top + space.xs }]} pointerEvents="box-none">
      <Gradient colors={gradients.top} />
      <View style={styles.headerRow} pointerEvents="box-none">
        <Text style={styles.headerTitle} color={onMedia.text} accessibilityRole="header">
          Shorts
        </Text>
        <Pressable onPress={onShuffle} hitSlop={8} accessibilityRole="button" accessibilityLabel="Shuffle a new feed" style={styles.round}>
          <Icon sf="shuffle" md="shuffle" size={20} color={onMedia.text} />
        </Pressable>
      </View>
      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.shelves}>
        {SHELVES.map((s) => (
          <Chip
            key={s.shelf}
            label={s.label}
            selected={shelf === s.shelf}
            accessibilityLabel={`Shuffle ${s.label === "Mix" ? "videos, anime and comics" : s.label.toLowerCase()}`}
            onPress={() => onPick(s.shelf)}
          />
        ))}
      </ScrollView>
    </View>
  );
}

type PageProps = {
  short: Short;
  height: number;
  near: boolean; // this page or a neighbour of the active one: it has a loaded player
  playing: boolean; // the active page, the tab in front, not paused
  onTogglePause: () => void;
};

// ShortPage is one screenful: the still, blurred, filling the screen behind the clip at its own
// shape, and the title and actions over the bottom.
function ShortPage({ short, height, near, playing, onTogglePause }: PageProps) {
  const { width } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const { media } = short;
  const thumb = thumbnailUrl(media);
  const videoHeight = width / Math.max(0.5, (media.video?.width ?? 16) / (media.video?.height || 9));
  const anime = media.section === Section.Anime;
  const comic = short.kind === "comic";

  return (
    <View style={[styles.page, { height }]}>
      {short.kind === "comic" ? (
        <ComicStory short={short} near={near} playing={playing} onTogglePause={onTogglePause} />
      ) : (
        <>
          {thumb && (
            <Image source={{ uri: thumb, headers: authHeaders() }} style={StyleSheet.absoluteFill} contentFit="cover" blurRadius={40} accessible={false} />
          )}
          <View style={[StyleSheet.absoluteFill, styles.dim]} />
          <View style={[styles.stage, { height: videoHeight, marginTop: -videoHeight * 0.15 }]}>
            {thumb && <Image source={{ uri: thumb, headers: authHeaders() }} style={StyleSheet.absoluteFill} contentFit="cover" accessible={false} />}
            {near && <Clip short={short} playing={playing} onTogglePause={onTogglePause} />}
          </View>
        </>
      )}

      <Gradient colors={gradients.posterBottom} style={styles.bottomFade} />
      {/* Clear of the floating tab bar: its height plus the home indicator. */}
      <View style={[styles.info, { paddingBottom: insets.bottom + 72 }]} pointerEvents="box-none">
        <Text variant="caption" color={onMedia.text} style={[styles.tag, (anime || comic) && styles.tagAnime]}>
          {comic ? "Comic" : anime ? "Anime" : "Video"}
        </Text>
        <Text style={styles.title} color={onMedia.text} numberOfLines={2}>
          {media.title}
        </Text>
        <Text variant="label" color={onMedia.muted} numberOfLines={1}>
          {detailLine(media)}
        </Text>
        <View style={styles.actions}>
          {comic ? (
            <Button
              label={resumePage(media) > 0 ? "Continue reading" : "Read"}
              icon={{ sf: "book.fill", md: "menu_book" }}
              variant="play"
              haptic
              onPress={() => router.push({ pathname: "/reader/[id]", params: { id: media.id } })}
              style={styles.watch}
            />
          ) : (
            <Button
              label={resumeAt(media) > 0 ? "Resume" : "Watch"}
              icon={{ sf: "play.fill", md: "play_arrow" }}
              variant="play"
              haptic
              onPress={() => router.push({ pathname: "/player/[id]", params: { id: media.id } })}
              style={styles.watch}
            />
          )}
          <Button
            label="Details"
            icon={{ sf: "info.circle", md: "info" }}
            variant="secondary"
            onPress={() => router.push({ pathname: "/media/[id]", params: { id: media.id } })}
            style={styles.watch}
          />
        </View>
      </View>
    </View>
  );
}

type ComicProps = { short: Extract<Short, { kind: "comic" }>; near: boolean; playing: boolean; onTogglePause: () => void };

// ComicStory is a comic's short: its late pages one after another, each full height over a blurred
// copy of itself, with a dash per page along the top like a story; the title and Read sit over the
// bottom of the page, on the same fade as a video's. It loops while it's the page on
// screen and holds still when paused or swiped away. The pages are fetched as it comes within one
// swipe, so the first is there when you land on it.
function ComicStory({ short, near, playing, onTogglePause }: ComicProps) {
  const insets = useSafeAreaInsets();
  const [n, setN] = useState(0);
  const count = short.pages.length;
  const uri = short.pages[n];

  useEffect(() => {
    if (near) void Image.prefetch(short.pages, { cachePolicy: "disk", headers: authHeaders() });
  }, [near, short.pages]);

  useEffect(() => {
    if (!playing) return;
    const t = setTimeout(() => setN((i) => (i + 1) % count), COMIC_PAGE_MS);
    return () => clearTimeout(t);
  }, [playing, n, count]);

  return (
    <>
      <Image source={{ uri, headers: authHeaders() }} style={StyleSheet.absoluteFill} contentFit="cover" blurRadius={40} accessible={false} />
      <View style={[StyleSheet.absoluteFill, styles.dim]} />
      <Pressable
        style={[StyleSheet.absoluteFill, styles.center, { paddingTop: insets.top + 112, paddingBottom: insets.bottom + 150 }]}
        onPress={onTogglePause}
        accessibilityRole="button"
        accessibilityLabel={playing ? "Pause" : "Play"}
      >
        <Image
          source={{ uri, headers: authHeaders() }}
          style={styles.comicPage}
          contentFit="contain"
          transition={180}
          accessible={false}
        />
        {!playing && near && (
          <View style={[styles.bigGlyph, styles.overPage]}>
            <Icon sf="play.fill" md="play_arrow" size={30} color={onMedia.text} />
          </View>
        )}
      </Pressable>
      {/* Page back and forward. Either one restarts the page's timer, so you get the full time on
          the page you chose; while paused, the story stays paused on it. */}
      <View style={[styles.pageButtons, { top: insets.top + 112, bottom: insets.bottom + 150 }]} pointerEvents="box-none">
        <Pressable
          onPress={() => setN((i) => (i - 1 + count) % count)}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Previous page, ${((n - 1 + count) % count) + 1} of ${count}`}
          style={({ pressed }) => [styles.round, pressed && styles.pressedRound]}
        >
          <Icon sf="chevron.left" md="chevron_left" size={20} color={onMedia.text} />
        </Pressable>
        <Pressable
          onPress={() => setN((i) => (i + 1) % count)}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Next page, ${((n + 1) % count) + 1} of ${count}`}
          style={({ pressed }) => [styles.round, pressed && styles.pressedRound]}
        >
          <Icon sf="chevron.right" md="chevron_right" size={20} color={onMedia.text} />
        </Pressable>
      </View>
      {/* Under the header's pills, above the page. */}
      <View style={[styles.storyDashes, { top: insets.top + 102 }]} pointerEvents="none">
        {short.pages.map((p, i) => (
          <View key={p} style={[styles.storyDash, { backgroundColor: i <= n ? onMedia.text : onMedia.track }]} />
        ))}
      </View>
    </>
  );
}

type ClipProps = { short: Extract<Short, { kind: "video" }>; playing: boolean; onTogglePause: () => void };

// Clip is one page's own player, created when the page comes within one swipe of the screen, so it
// has loaded by the time you land on it. The video stays invisible over the still until it has
// drawn its own first frame. It loops; leaving it rewinds it, so coming back starts the clip again.
function Clip({ short, playing, onTogglePause }: ClipProps) {
  const isMuted = useMuted();
  const source = useMemo(() => ({ uri: short.uri, headers: authHeaders(), metadata: { title: short.media.title } }), [short]);
  const player = useVideoPlayer(source, (p) => {
    p.loop = true;
    p.muted = muted;
    p.timeUpdateEventInterval = 0.25;
    p.audioMixingMode = "auto";
  });
  const [drawn, setDrawn] = useState(false);
  const s = usePlayerStatus(player);

  useEffect(() => {
    if (playing) {
      player.play();
      return;
    }
    player.pause();
    rewind(player);
  }, [playing, player]);

  useEffect(() => applyMuted(player, isMuted), [isMuted, player]);

  return (
    <>
      <VideoView
        player={player}
        style={[StyleSheet.absoluteFill, { opacity: drawn ? 1 : 0 }]}
        contentFit="cover"
        nativeControls={false}
        allowsPictureInPicture={false}
        onFirstFrameRender={() => setDrawn(true)}
      />
      <Pressable
        style={[StyleSheet.absoluteFill, styles.center]}
        onPress={onTogglePause}
        accessibilityRole="button"
        accessibilityLabel={playing ? "Pause" : "Play"}
      >
        {!playing && drawn && (
          <View style={styles.bigGlyph}>
            <Icon sf="play.fill" md="play_arrow" size={30} color={onMedia.text} />
          </View>
        )}
      </Pressable>
      {short.media.video?.short && (
        <Pressable
          onPress={() => setMuted(!isMuted)}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={isMuted ? "Turn sound on" : "Turn sound off"}
          style={[styles.round, styles.mute]}
        >
          <Icon sf={isMuted ? "speaker.slash.fill" : "speaker.wave.2.fill"} md={isMuted ? "volume_off" : "volume_up"} size={16} color={onMedia.text} />
        </Pressable>
      )}
      {playing && <Progress current={s.currentTime} duration={s.duration} />}
    </>
  );
}

// Progress is a hairline along the bottom of the clip: how far into this montage you are.
function Progress({ current, duration }: { current: number; duration: number }) {
  const { colors } = useAppTheme();
  const done = duration > 0 ? Math.min(1, current / duration) : 0;
  return (
    <View style={[styles.progress, { backgroundColor: onMedia.track }]} pointerEvents="none">
      <View style={{ width: `${done * 100}%`, height: "100%", backgroundColor: colors.primary }} />
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: onMedia.backdrop },
  header: { position: "absolute", top: 0, left: 0, right: 0, paddingBottom: space.lg, gap: space.sm },
  headerRow: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingHorizontal: space.lg },
  headerTitle: { ...typography.display },
  shelves: { paddingHorizontal: space.lg, gap: space.sm },
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
  mute: { position: "absolute", right: space.md, bottom: space.md + 4 },
  progress: { position: "absolute", left: 0, right: 0, bottom: 0, height: 3 },
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
  comicPage: { width: "100%", height: "100%" },
  overPage: { position: "absolute" },
  pageButtons: { position: "absolute", left: space.sm, right: space.sm, flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  pressedRound: { opacity: 0.6 },
  storyDashes: { position: "absolute", left: space.lg, right: space.lg, flexDirection: "row", gap: 3 },
  storyDash: { flex: 1, height: 2, borderRadius: 1 },
});
