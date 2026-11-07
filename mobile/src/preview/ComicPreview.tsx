import { Image } from "expo-image";
import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import { StyleSheet, View } from "react-native";

import { authHeaders } from "@/api/client";
import { pageUrl, type Media } from "@/api/media";
import { motion, onMedia, space } from "@/theme";
import { stopPreview } from "./store";

// How many pages a preview flips through, and how long each stays up: long enough to take in a
// page's layout and colour, short enough that the whole flip is a glance.
export const COMIC_PREVIEW_PAGES = 6;
const PAGE_MS = 850;

// ComicPreview is a comic's long-press preview: its first pages, one after another, over the
// cover, then the cover again. The comic counterpart of a video's montage, drawn over the poster
// only while this card is the one previewing. It ends early on blur or unmount, like PreviewVideo.
export function ComicPreview({ comic }: { comic: Media }) {
  const count = Math.min(COMIC_PREVIEW_PAGES, comic.comic?.pageCount ?? 0);
  const urls = useMemo(() => Array.from({ length: count }, (_, n) => pageUrl(comic, n)), [comic, count]);
  // Page 0 is the cover, already on screen: the flip starts at page 1.
  const [n, setN] = useState(1);

  useEffect(() => {
    void Image.prefetch(urls.slice(1), { cachePolicy: "disk", headers: authHeaders() });
  }, [urls]);

  useEffect(() => {
    const t = setTimeout(() => (n + 1 < count ? setN(n + 1) : stopPreview(comic.id)), PAGE_MS);
    return () => clearTimeout(t);
  }, [n, count, comic.id]);

  useFocusEffect(useCallback(() => () => stopPreview(comic.id), [comic.id]));
  useEffect(() => () => stopPreview(comic.id), [comic.id]);

  if (count < 2) return null;
  return (
    <View style={[StyleSheet.absoluteFill, { backgroundColor: onMedia.backdrop }]} pointerEvents="none">
      <Image
        source={{ uri: urls[n], headers: authHeaders() }}
        style={StyleSheet.absoluteFill}
        contentFit="contain"
        transition={motion.fast}
        accessible={false}
      />
      {/* Where the flip is: one dash per page, like a story's progress. */}
      <View style={styles.dots}>
        {urls.slice(1).map((u, i) => (
          <View key={u} style={[styles.dot, { backgroundColor: i + 1 <= n ? onMedia.text : onMedia.track }]} />
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  dots: { position: "absolute", top: space.sm, left: space.sm, right: space.sm, flexDirection: "row", gap: 3 },
  dot: { flex: 1, height: 2, borderRadius: 1 },
});
