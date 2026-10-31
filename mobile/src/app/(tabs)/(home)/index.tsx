import { router, useFocusEffect } from "expo-router";
import { useCallback } from "react";
import { FlatList, ScrollView, StyleSheet, View } from "react-native";

import { listMedia, MediaKind, type Media } from "@/api/media";
import { getContinue } from "@/api/progress";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { MediaCard } from "@/components/MediaCard";
import { Skeleton } from "@/components/Skeleton";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { space } from "@/theme";

const RECENT_COUNT = 10;
const CARD_WIDTH = 160; // a 16:9 video poster
const COVER_WIDTH = 110; // a 2:3 comic cover is tall, so it gets a narrower card

// Defined outside the component: the same function every render, so neither the list
// nor its rows re-render just because Home did.
// Each row asks the server for exactly what it shows: the newest few of one kind, by when Velora
// first saw them. (Before phase 12 Home fetched the whole library and sorted it here.)
const fetchHome = async (signal: AbortSignal) => {
  const recent = (kind: MediaKind) => listMedia({ kind, sort: "recent", limit: RECENT_COUNT, signal });
  const [videos, comics, cont] = await Promise.all([recent(MediaKind.Video), recent(MediaKind.Comic), getContinue({ signal })]);
  return { videos: videos.items, comics: comics.items, ...cont };
};
const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });
const resume = (m: Media) =>
  router.push({ pathname: m.kind === MediaKind.Video ? "/player/[id]" : "/reader/[id]", params: { id: m.id } });
// Each row holds one kind, so the cards don't need a VIDEO / COMIC badge.
const renderCard = ({ item }: { item: Media }) => (
  <MediaCard media={item} onPress={openMedia} style={item.kind === MediaKind.Comic ? styles.cover : styles.card} showKind={false} />
);
const renderResume = ({ item }: { item: Media }) => (
  <MediaCard media={item} onPress={resume} style={item.kind === MediaKind.Comic ? styles.cover : styles.card} showKind={false} />
);

export default function Home() {
  const { state, refetch } = useAsync(fetchHome);

  // Coming back from the player or the reader: the Continue rows have changed.
  useFocusEffect(useCallback(() => refetch(), [refetch]));

  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;

  return (
    // automatic: content starts below the large title and scrolls under the (glass) header.
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={styles.content}>
      {state.status === "loading" ? (
        <HomeSkeleton />
      ) : state.data.videos.length === 0 && state.data.comics.length === 0 ? (
        <EmptyView
          title="Your library is empty"
          detail="Put videos in movies/ and comics in comics/ inside the folder velora-server serves, then open Videos or Comics and pull down to rescan."
          action={{ label: "Open Videos", onPress: () => router.push("/videos") }}
        />
      ) : (
        <>
          <Row title="Continue watching" items={state.data.watching} render={renderResume} />
          <Row title="Continue reading" items={state.data.reading} render={renderResume} />
          <Row title="New videos" items={state.data.videos} render={renderCard} />
          <Row title="New comics" items={state.data.comics} render={renderCard} />
        </>
      )}
    </ScrollView>
  );
}

type RowProps = { title: string; items: Media[]; render: (info: { item: Media }) => React.ReactElement };

// Row is a heading and a horizontal list; an empty row isn't drawn at all.
function Row({ title, items, render }: RowProps) {
  if (items.length === 0) return null;
  return (
    <View style={styles.section}>
      <Text variant="title" accessibilityRole="header" style={styles.heading}>
        {title}
      </Text>
      <FlatList
        horizontal
        data={items}
        keyExtractor={(m) => m.id}
        renderItem={render}
        contentContainerStyle={styles.row}
        showsHorizontalScrollIndicator={false}
      />
    </View>
  );
}

// HomeSkeleton has the shape of one row of cards: a heading and three posters.
function HomeSkeleton() {
  return (
    <View style={styles.section} accessibilityLabel="Loading">
      <Skeleton width={180} height={24} style={styles.heading} />
      <View style={styles.row}>
        {[0, 1, 2].map((i) => (
          <View key={i} style={styles.card}>
            <Skeleton aspectRatio={16 / 9} />
            <Skeleton width="70%" height={16} style={styles.skeletonTitle} />
          </View>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  content: { paddingVertical: space.lg, gap: space.xl, flexGrow: 1 },
  section: { gap: space.md },
  heading: { marginHorizontal: space.lg },
  row: { flexDirection: "row", paddingHorizontal: space.lg, gap: space.md },
  card: { width: CARD_WIDTH },
  cover: { width: COVER_WIDTH },
  skeletonTitle: { marginTop: space.sm },
});

