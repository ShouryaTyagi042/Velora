import { Image } from "expo-image";
import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import { StyleSheet, View } from "react-native";

import { authHeaders } from "@/api/client";
import { pageUrl, type Media } from "@/api/media";
import { latePages } from "@/lib/pages";
import { motion, onMedia, space } from "@/theme";
import { stopPreview } from "./store";

// How many pages a preview flips through, and how long each stays up: long enough to take in a
// page's layout and colour, short enough that the whole flip is a glance.
export const COMIC_PREVIEW_PAGES = 6;
const PAGE_MS = 850;

// ComicPreview is a comic's long-press preview: pages from past 60% of it (latePages), one after
// another, over the cover, then the cover again. The comic counterpart of a video's montage,
// drawn over the poster only while this card is the one previewing. It ends early on blur or
// unmount, like PreviewVideo.
export function ComicPreview({ comic }: { comic: Media }) {
  const urls = useMemo(() => latePages(comic.comic?.pageCount ?? 0, COMIC_PREVIEW_PAGES).map((p) => pageUrl(comic, p)), [comic]);
  const count = urls.length;
  const [n, setN] = useState(0);

  useEffect(() => {
    void Image.prefetch(urls, { cachePolicy: "disk", headers: authHeaders() });
  }, [urls]);

  useEffect(() => {
    const t = setTimeout(() => (n + 1 < count ? setN(n + 1) : stopPreview(comic.id)), PAGE_MS);
    return () => clearTimeout(t);
  }, [n, count, comic.id]);

  useFocusEffect(useCallback(() => () => stopPreview(comic.id), [comic.id]));
  useEffect(() => () => stopPreview(comic.id), [comic.id]);

  if (count === 0) return null;
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
        {urls.map((u, i) => (
          <View key={u} style={[styles.dot, { backgroundColor: i <= n ? onMedia.text : onMedia.track }]} />
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  dots: { position: "absolute", top: space.sm, left: space.sm, right: space.sm, flexDirection: "row", gap: 3 },
  dot: { flex: 1, height: 2, borderRadius: 1 },
});
