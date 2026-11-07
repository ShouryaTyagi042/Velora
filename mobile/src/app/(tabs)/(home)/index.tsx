import { router, useFocusEffect } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { Pressable, RefreshControl, ScrollView, StyleSheet, useWindowDimensions, View } from "react-native";
import Animated, { interpolate, useAnimatedScrollHandler, useAnimatedStyle, useSharedValue } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { listActors, listMedia, listSeries, MediaKind, randomMedia, Section, type ActorCount, type Media, type SeriesCount } from "@/api/media";
import { getContinue } from "@/api/progress";
import { SignOutButton } from "@/auth/SignOutButton";
import { Button } from "@/components/Button";
import { Chip } from "@/components/Chip";
import { CoverCard } from "@/components/CoverCard";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Gradient } from "@/components/Gradient";
import { MediaCard } from "@/components/MediaCard";
import { Poster } from "@/components/Poster";
import { Rail } from "@/components/Rail";
import { Skeleton } from "@/components/Skeleton";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { plural } from "@/lib/format";
import { openMedia } from "@/navigation/openMedia";
import { PreviewVideo } from "@/preview/PreviewVideo";
import { startPreview, stopPreview, useIsPreviewing } from "@/preview/store";
import { resumeAt } from "@/progress/resume";
import { gradients, onMedia, space, typography, useAppTheme, wordmark } from "@/theme";

const RECENT = 12;
const RAIL_CARD = 168; // a 16:9 still in a rail
const RAIL_COVER = 112; // a 2:3 comic cover is tall, so it gets a narrower card
const RAIL_GROUP = 200; // an actor's or a series' tile
// The billboard is a little taller than wide: a poster's proportions, cropped from a 16:9 still.
const BILLBOARD_TALL = 1.18;
// How long the billboard shows its still before its preview starts playing over it.
const TEASER_DELAY = 1500;

// The billboard is one video at random, preferring one that is processed (it has a still, and
// maybe a teaser). Picked when Home opens and again when you pull down to refresh, but not on
// every return to it: coming back from the player to a different billboard would feel like
// losing your place.
const fetchBillboard = async (signal: AbortSignal) => {
  const picks = await randomMedia(6, { kind: MediaKind.Video, signal });
  return picks.find((m) => m.video?.preview) ?? picks.find((m) => m.video) ?? picks[0] ?? null;
};

// Everything else Home shows, in one round trip's time: each rail asks the server for exactly
// what it draws.
const fetchHome = async (signal: AbortSignal) => {
  const recent = (kind: MediaKind, section?: Section) => listMedia({ kind, section, sort: "recent", limit: RECENT, signal });
  const [films, anime, comics, cont, actors, series] = await Promise.all([
    recent(MediaKind.Video, Section.Movies),
    recent(MediaKind.Video, Section.Anime),
    recent(MediaKind.Comic),
    getContinue({ signal }),
    listActors({ section: Section.Movies, signal }),
    listSeries({ signal }),
  ]);
  return {
    films: films.items,
    anime: anime.items,
    comics: comics.items,
    ...cont,
    // The people and shows with the most in the library come first on Home.
    actors: [...actors].sort((a, b) => b.videoCount - a.videoCount).slice(0, RECENT),
    series: series.slice(0, RECENT),
  };
};

const play = (m: Media) =>
  router.push({ pathname: m.kind === MediaKind.Video ? "/player/[id]" : "/reader/[id]", params: { id: m.id } });
const details = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });

// The rails' cards. Defined outside the component: the same functions every render.
const still = (m: Media) => <MediaCard media={m} onPress={openMedia} style={styles.card} />;
const resume = (m: Media) => <MediaCard media={m} onPress={play} style={m.kind === MediaKind.Comic ? styles.cover : styles.card} />;
const cover = (m: Media) => <MediaCard media={m} onPress={openMedia} style={styles.cover} />;
const actor = (a: ActorCount) => (
  <CoverCard
    layout="tile"
    name={a.name}
    count={plural(a.videoCount, "video")}
    cover={a.cover}
    icon="person"
    style={styles.group}
    onPress={() => router.push({ pathname: "/actor/[name]", params: { name: a.name } })}
  />
);
const show = (s: SeriesCount) => (
  <CoverCard
    layout="tile"
    name={s.name}
    count={plural(s.episodeCount, "episode")}
    cover={s.cover}
    icon="tv"
    style={styles.group}
    onPress={() => router.push({ pathname: "/series/[name]", params: { name: s.name } })}
  />
);

// Home: a billboard of one thing to watch now, then rails to browse, the way a streaming service
// opens. The header floats over the billboard and turns solid as the page scrolls under it.
export default function Home() {
  const { colors } = useAppTheme();
  const { state, refetch, refreshing } = useAsync(fetchHome);
  const { state: billboard, refetch: reshuffle, refreshing: reshuffling } = useAsync(fetchBillboard);
  // Only a pull shows the spinner: the quiet refetch on returning to Home doesn't.
  const [pulled, setPulled] = useState(false);
  const scrollY = useSharedValue(0);
  const onScroll = useAnimatedScrollHandler((e) => {
    scrollY.set(e.contentOffset.y);
  });

  // Coming back from the player or the reader: the Continue rails have changed.
  useFocusEffect(
    useCallback(() => {
      setPulled(false);
      refetch();
    }, [refetch]),
  );

  // Pull to refresh: every rail again, and a new billboard.
  const refresh = () => {
    setPulled(true);
    refetch();
    reshuffle();
  };

  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  const d = state.status === "ok" && billboard.status !== "loading" ? state.data : null;
  const pick = billboard.status === "ok" ? billboard.data : null;
  const empty = d && d.films.length === 0 && d.anime.length === 0 && d.comics.length === 0;

  return (
    <View style={styles.screen}>
      <Animated.ScrollView
        onScroll={onScroll}
        scrollEventThrottle={16}
        contentInsetAdjustmentBehavior="never"
        contentContainerStyle={styles.content}
        showsVerticalScrollIndicator={false}
        refreshControl={
          <RefreshControl
            refreshing={pulled && (refreshing || reshuffling)}
            onRefresh={refresh}
            tintColor={colors.text}
            colors={[colors.primary]}
            progressBackgroundColor={colors.surface}
          />
        }
      >
        {!d ? (
          <HomeSkeleton />
        ) : empty ? (
          <View style={styles.emptyBox}>
            <EmptyView
              title="Your library is empty"
              detail="Put films in movies/, anime in anime/ and comics in comics/ inside the folder velora-server serves, then pull down on Actors to rescan."
              action={{ label: "Open Actors", onPress: () => router.navigate("/actors") }}
            />
          </View>
        ) : (
          <>
            {pick ? <Billboard media={pick} /> : <View style={styles.headerSpace} />}
            <View style={styles.rails}>
              <Rail title="Continue watching" items={d.watching} keyOf={(m) => m.id} render={resume} />
              <Rail title="New films" items={d.films} keyOf={(m) => m.id} render={still} seeAll="/videos" />
              <Rail title="Actors" items={d.actors} keyOf={(a) => a.name} render={actor} seeAll="/actors" />
              <Rail title="New anime" items={d.anime} keyOf={(m) => m.id} render={still} seeAll="/episodes" />
              <Rail title="Series" items={d.series} keyOf={(s) => s.name} render={show} seeAll="/anime" />
              <Rail title="Continue reading" items={d.reading} keyOf={(m) => m.id} render={resume} />
              <Rail title="New comics" items={d.comics} keyOf={(m) => m.id} render={cover} seeAll="/comics" />
            </View>
          </>
        )}
      </Animated.ScrollView>
      <HomeHeader scrollY={scrollY} />
    </View>
  );
}

// HomeHeader: the wordmark and the shortcuts. Transparent over the billboard, with a fade so it
// stays legible; once the billboard has scrolled away it settles onto a solid bar.
function HomeHeader({ scrollY }: { scrollY: { get: () => number } }) {
  const { colors } = useAppTheme();
  const insets = useSafeAreaInsets();
  const solid = useAnimatedStyle(() => ({ opacity: interpolate(scrollY.get(), [0, 220], [0, 1], "clamp") }));
  // Pulling down to refresh fades the header away, so the spinner under it shows.
  const pulling = useAnimatedStyle(() => ({ opacity: interpolate(scrollY.get(), [-70, -10], [0, 1], "clamp") }));
  return (
    <Animated.View style={[styles.header, { paddingTop: insets.top }, pulling]} pointerEvents="box-none">
      <Gradient colors={gradients.top} />
      <Animated.View style={[StyleSheet.absoluteFill, { backgroundColor: colors.background }, solid]} />
      <View style={styles.brandRow}>
        <Text style={wordmark} color={colors.primaryText} accessibilityRole="header" accessibilityLabel="Velora">
          VELORA
        </Text>
        <SignOutButton />
      </View>
      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.pills}>
        <Chip label="Actors" onPress={() => router.navigate("/actors")} />
        <Chip label="Anime" onPress={() => router.navigate("/anime")} />
        <Chip label="Comics" onPress={() => router.push("/comics")} />
        <Chip label="Surprise me" featured onPress={() => router.push({ pathname: "/surprise", params: { kind: "any" } })} />
      </ScrollView>
    </Animated.View>
  );
}

// Billboard: one video, full width, its still melting into the page through a blue dusk, its title
// set large, Play and Details under it. After a moment its preview plays over the still, muted.
function Billboard({ media }: { media: Media }) {
  const previewing = useIsPreviewing(media.id);
  const anime = media.section === Section.Anime;
  const at = resumeAt(media);

  useFocusEffect(
    useCallback(() => {
      if (!media.video?.preview) return;
      const t = setTimeout(() => startPreview(media), TEASER_DELAY);
      return () => {
        clearTimeout(t);
        stopPreview(media.id);
      };
    }, [media]),
  );
  useEffect(() => () => stopPreview(media.id), [media.id]);

  const kicker = [anime ? "Anime" : "Film", media.series ?? media.actors?.[0]?.name].filter(Boolean).join("  ·  ");
  return (
    <View>
      <Pressable onPress={() => details(media)} accessibilityRole="button" accessibilityLabel={`${media.title}, details`}>
        <Poster media={media} aspectRatio={1 / BILLBOARD_TALL} style={styles.billboardArt}>
          {previewing && <PreviewVideo id={media.id} />}
          <Gradient colors={gradients.billboard} style={styles.billboardFade} />
        </Poster>
      </Pressable>
      <View style={styles.billboardText} pointerEvents="box-none">
        <Text variant="label" color={onMedia.muted} style={styles.center}>
          {kicker}
        </Text>
        <Text style={[typography.hero, styles.center]} color={onMedia.text} numberOfLines={3}>
          {media.title}
        </Text>
        <View style={styles.billboardActions}>
          <Button
            label={at > 0 ? "Resume" : "Play"}
            icon={{ sf: "play.fill", md: "play_arrow" }}
            variant="play"
            haptic
            onPress={() => play(media)}
            style={styles.billboardButton}
          />
          <Button
            label="Details"
            icon={{ sf: "info.circle", md: "info" }}
            variant="secondary"
            onPress={() => details(media)}
            style={styles.billboardButton}
          />
        </View>
      </View>
    </View>
  );
}

// HomeSkeleton has the page's shape: the billboard, then a rail.
function HomeSkeleton() {
  const { width } = useWindowDimensions();
  return (
    <View accessibilityLabel="Loading">
      <Skeleton height={width * BILLBOARD_TALL} style={styles.billboardArt} />
      <View style={[styles.rails, styles.skeletonRail]}>
        <Skeleton width={160} height={22} />
        <View style={styles.skeletonRow}>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} width={RAIL_CARD} aspectRatio={16 / 9} />
          ))}
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  content: { paddingBottom: space.xxxl * 2, flexGrow: 1 },
  emptyBox: { flex: 1, minHeight: 600, paddingTop: 140 },
  headerSpace: { height: 150 },
  header: { position: "absolute", top: 0, left: 0, right: 0, paddingBottom: space.sm },
  brandRow: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingLeft: space.lg, paddingRight: space.sm },
  pills: { paddingHorizontal: space.lg, gap: space.sm, paddingTop: space.xs, paddingBottom: space.xs },
  billboardArt: { borderRadius: 0 },
  billboardFade: { top: "35%" },
  billboardText: { position: "absolute", left: 0, right: 0, bottom: 0, paddingHorizontal: space.xl, gap: space.sm, alignItems: "center" },
  center: { textAlign: "center" },
  billboardActions: { flexDirection: "row", gap: space.sm, marginTop: space.sm, alignSelf: "stretch" },
  billboardButton: { flex: 1 },
  rails: { gap: space.xl + space.xs, marginTop: space.xl },
  card: { width: RAIL_CARD },
  cover: { width: RAIL_COVER },
  group: { width: RAIL_GROUP },
  skeletonRail: { paddingHorizontal: space.lg, gap: space.md },
  skeletonRow: { flexDirection: "row", gap: space.sm },
});
