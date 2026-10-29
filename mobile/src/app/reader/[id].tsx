import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback, useMemo, useState } from "react";
import { FlatList, Pressable, StyleSheet, Text, useWindowDimensions, View, type NativeScrollEvent, type NativeSyntheticEvent } from "react-native";
import Animated, { useAnimatedStyle, useReducedMotion, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError } from "@/api/client";
import { MediaKind, pageUrl, type Media } from "@/api/media";
import { ComicPage } from "@/components/ComicPage";
import { ErrorView } from "@/components/ErrorView";
import { Icon } from "@/components/Icon";
import { Loading } from "@/components/Loading";
import { Surface } from "@/components/Surface";
import { useAsync } from "@/hooks/useAsync";
import { usePreload } from "@/hooks/usePreload";
import { loadWithProgress, resumePage } from "@/progress/resume";
import { useProgressSync } from "@/progress/useProgressSync";
import { useReaderSettings } from "@/reader/settings";
import { onMedia, space, touch, typography } from "@/theme";

const FG = onMedia.text; // controls sit over pages on a black background

export default function ReaderScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback((signal: AbortSignal) => loadWithProgress(id, signal), [id]);
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  if (state.data.kind !== MediaKind.Comic || !state.data.comic) {
    return <ErrorView error={new ApiError(404, "not_found", `${state.data.title} is not a comic.`)} />;
  }
  return <Reader comic={state.data} pageCount={state.data.comic.pageCount} />;
}

function Reader({ comic, pageCount }: { comic: Media; pageCount: number }) {
  const { width, height } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const reduceMotion = useReducedMotion();
  const { direction, setDirection } = useReaderSettings();
  const horizontal = direction === "horizontal";

  const [page, setPage] = useState(() => resumePage(comic));
  const sync = useProgressSync(comic.id);
  const [chrome, setChrome] = useState(true); // the top and bottom bars
  const [zoomed, setZoomed] = useState(false);

  const urls = useMemo(() => Array.from({ length: pageCount }, (_, n) => pageUrl(comic, n)), [comic, pageCount]);
  usePreload(urls, page);

  // Every page is exactly one screen, so the index is the offset divided by the page size.
  const size = horizontal ? width : height;
  const onScrollEnd = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const offset = horizontal ? e.nativeEvent.contentOffset.x : e.nativeEvent.contentOffset.y;
    const n = Math.min(pageCount - 1, Math.max(0, Math.round(offset / size)));
    if (n === page) return;
    setPage(n);
    // A page turn is a discrete event, not a stream: save each one (one request in flight at a time).
    sync.save({ page: n, pageCount }, { force: true });
  };

  const fade = useAnimatedStyle(() => ({
    opacity: withTiming(chrome ? 1 : 0, { duration: reduceMotion ? 0 : 200 }),
  }));

  return (
    <View style={styles.screen}>
      <Stack.Screen options={{ headerShown: false }} />

      <FlatList
        // A new key when the direction changes: the list is rebuilt rather than re-laid-out
        // mid-scroll, and opens on the page you were on.
        key={direction}
        data={urls}
        keyExtractor={(u) => u}
        horizontal={horizontal}
        pagingEnabled
        scrollEnabled={!zoomed}
        showsHorizontalScrollIndicator={false}
        showsVerticalScrollIndicator={false}
        initialScrollIndex={page}
        getItemLayout={(_, index) => ({ length: size, offset: size * index, index })}
        // Only the page on screen and its neighbours are mounted: a 300-page comic is 3 images.
        windowSize={3}
        initialNumToRender={1}
        maxToRenderPerBatch={2}
        onMomentumScrollEnd={onScrollEnd}
        renderItem={({ item, index }) => (
          <ComicPage
            uri={item}
            width={width}
            height={height}
            zoomed={zoomed && index === page}
            onZoomChange={setZoomed}
            onTap={() => setChrome((c) => !c)}
            label={`Page ${index + 1} of ${pageCount}`}
          />
        )}
      />

      <Animated.View style={[styles.top, { paddingTop: insets.top + space.xs }, fade]} pointerEvents={chrome ? "box-none" : "none"}>
        <Pressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Close reader" hitSlop={8}>
          <Surface role="control" visible={chrome} colorScheme="dark" style={styles.round}>
            <Icon sf="xmark" md="close" size={18} color={FG} />
          </Surface>
        </Pressable>
        <Surface role="bar" visible={chrome} colorScheme="dark" style={styles.titleBar}>
          <Text style={styles.title} numberOfLines={1}>
            {comic.title}
          </Text>
        </Surface>
        <Pressable
          onPress={() => setDirection(horizontal ? "vertical" : "horizontal")}
          accessibilityRole="button"
          accessibilityLabel={horizontal ? "Read top to bottom" : "Read left to right"}
          hitSlop={8}
        >
          <Surface role="control" visible={chrome} colorScheme="dark" style={styles.round}>
            {horizontal ? (
              <Icon sf="arrow.up.and.down" md="swap_vert" size={18} color={FG} />
            ) : (
              <Icon sf="arrow.left.and.right" md="swap_horiz" size={18} color={FG} />
            )}
          </Surface>
        </Pressable>
      </Animated.View>

      <Animated.View style={[styles.bottom, { paddingBottom: insets.bottom + space.sm }, fade]} pointerEvents="none">
        <Surface role="bar" visible={chrome} colorScheme="dark" style={styles.indicator}>
          <Text style={styles.indicatorText} accessibilityLiveRegion="polite">
            {page + 1} / {pageCount}
          </Text>
        </Surface>
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: onMedia.backdrop },
  top: {
    position: "absolute",
    left: 0,
    right: 0,
    top: 0,
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
    paddingHorizontal: space.md,
  },
  round: { width: touch.min, height: touch.min, borderRadius: touch.min / 2, alignItems: "center", justifyContent: "center", overflow: "hidden" },
  titleBar: { flex: 1, height: touch.min, borderRadius: touch.min / 2, justifyContent: "center", paddingHorizontal: space.lg, overflow: "hidden" },
  title: { ...typography.label, color: FG, fontWeight: "600", textAlign: "center" },
  bottom: { position: "absolute", left: 0, right: 0, bottom: 0, alignItems: "center" },
  indicator: { paddingHorizontal: space.md, height: 32, borderRadius: 16, justifyContent: "center", overflow: "hidden" },
  indicatorText: { ...typography.caption, color: FG, fontVariant: ["tabular-nums"] },
});
