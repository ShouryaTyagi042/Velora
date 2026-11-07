import { router, useFocusEffect, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ScrollView, StyleSheet, View } from "react-native";

import { listTags } from "@/api/media";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Skeleton } from "@/components/Skeleton";
import { TagChip } from "@/components/TagChip";
import { Text } from "@/components/Text";
import { TypeFilter } from "@/components/TypeFilter";
import { useAsync } from "@/hooks/useAsync";
import { plural } from "@/lib/format";
import { parseMediaType, typeFilter, VIDEO_TYPES } from "@/lib/mediaType";
import { space } from "@/theme";

// Tags: every tag in the library, each with how many videos have it. The most used come first,
// so the tags that organise the library are the first you see; tapping one lists its videos.
export default function Tags() {
  // ?type=videos|anime counts only that shelf's videos (tags are on videos, so no Comics).
  const type = parseMediaType(useLocalSearchParams<{ type?: string }>().type, VIDEO_TYPES);
  const fetchTags = useCallback((signal: AbortSignal) => listTags({ section: typeFilter(type).section, signal }), [type]);
  const { state, refetch } = useAsync(fetchTags);
  useFocusEffect(useCallback(() => refetch(), [refetch]));

  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  const tags = state.status === "ok" ? [...state.data].sort((a, b) => b.videoCount - a.videoCount || a.name.localeCompare(b.name)) : [];

  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={styles.content}>
      <TypeFilter value={type} options={VIDEO_TYPES} />
      {state.status === "ok" && tags.length === 0 ? (
        <View style={styles.empty}>
          <EmptyView
            title={type === "all" ? "No tags yet" : type === "anime" ? "No anime is tagged yet" : "No videos are tagged yet"}
            detail="Open a video and add a tag under Tags: a word or two that says what it is. Tags show up here and in Search."
            action={{ label: "Browse videos", onPress: () => router.navigate("/actors") }}
          />
        </View>
      ) : state.status === "loading" ? (
        <View style={styles.cloud} accessibilityLabel="Loading">
          {[80, 120, 64, 100, 90, 72].map((w, i) => (
            <Skeleton key={i} width={w} height={32} />
          ))}
        </View>
      ) : (
        <>
          <Text muted>{plural(tags.length, "tag")}, most used first.</Text>
          <View style={styles.cloud}>
            {tags.map((t) => (
              <TagChip key={t.name} name={t.name} count={t.videoCount} type={type} />
            ))}
          </View>
        </>
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  content: { padding: space.lg, gap: space.lg },
  cloud: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  empty: { minHeight: 360 },
});
