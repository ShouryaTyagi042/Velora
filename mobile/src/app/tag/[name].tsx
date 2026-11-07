import { Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { StyleSheet, View } from "react-native";

import { listMedia, MediaKind } from "@/api/media";
import { TypeFilter } from "@/components/TypeFilter";
import { VideoList } from "@/components/VideoList";
import { parseMediaType, typeFilter, VIDEO_TYPES } from "@/lib/mediaType";
import { space } from "@/theme";

// A tag's page: every video with the tag, newest first, one per row, and All / Videos / Anime to
// narrow it (?type=). Over the tabs, like Details: a tag is reached from a video, Search or the
// tag list, and back returns there.
export default function Tag() {
  const params = useLocalSearchParams<{ name: string; type?: string }>();
  const name = params.name;
  const type = parseMediaType(params.type, VIDEO_TYPES);
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) =>
      listMedia({ kind: MediaKind.Video, ...typeFilter(type), tag: name, sort: "recent", after, signal }),
    [name, type],
  );
  return (
    <>
      <Stack.Screen options={{ title: `#${name}` }} />
      <VideoList
        fetchPage={fetchPage}
        header={
          <View style={styles.header}>
            <TypeFilter value={type} options={VIDEO_TYPES} />
          </View>
        }
        empty={{
          title: type === "all" ? `Nothing tagged “${name}”` : `No ${type === "anime" ? "anime" : "videos"} tagged “${name}”`,
          detail: "Tag a video from its details page, under Tags.",
        }}
      />
    </>
  );
}

const styles = StyleSheet.create({
  header: { marginBottom: space.lg },
});
