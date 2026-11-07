import { Stack } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { ActivityIndicator, FlatList, Pressable, StyleSheet, View } from "react-native";
import type { SearchBarCommands } from "react-native-screens";

import { listMedia, listTags, type Media, type Page } from "@/api/media";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Icon } from "@/components/Icon";
import { Loading } from "@/components/Loading";
import { MediaCard } from "@/components/MediaCard";
import { TagChip } from "@/components/TagChip";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { usePages } from "@/hooks/usePages";
import { openMedia } from "@/navigation/openMedia";
import { clearHistory, forget, remember, useSearchHistory } from "@/search/history";
import { space, touch, useAppTheme } from "@/theme";

const nothing: Page<Media> = { items: [], nextCursor: null };

// SEARCH_DELAY is how long typing must pause before a search goes out.
const SEARCH_DELAY = 300;

// Search: the native search field (in the glass tab bar on iOS 26, in the header elsewhere). The
// server matches titles, actors' names and anime series, ignoring case and accents. Results are one
// per row, like every list of videos. With the field empty, your recent searches: tap one to run it
// again.
export default function Search() {
  const [query, setQuery] = useState("");
  const [q, setQ] = useState(""); // the query, once typing pauses
  const bar = useRef<SearchBarCommands>(null);
  useEffect(() => {
    const t = setTimeout(() => setQ(query.trim()), SEARCH_DELAY);
    return () => clearTimeout(t);
  }, [query]);

  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => (q ? listMedia({ q, after, signal }) : Promise.resolve(nothing)),
    [q],
  );
  const { state, refetch, loadMore, loadingMore } = usePages(fetchPage);

  // Running a recent search: put it in the field and search at once, without the typing delay.
  const rerun = (text: string) => {
    bar.current?.setText(text);
    setQuery(text);
    setQ(text);
    remember(text);
  };
  const open = (m: Media) => {
    remember(q);
    openMedia(m);
  };

  const header = (
    <Stack.Screen
      options={{
        headerSearchBarOptions: {
          ref: bar,
          placeholder: "Titles, actors, series and tags",
          onChangeText: (e) => setQuery(e.nativeEvent.text),
          onSearchButtonPress: (e) => remember(e.nativeEvent.text),
          onCancelButtonPress: () => setQuery(""),
          autoCapitalize: "none",
        },
      }}
    />
  );

  if (!query.trim()) return <>{header}<History onRun={rerun} /></>;
  if (state.status === "loading") return <>{header}<Loading /></>;
  if (state.status === "error") return <>{header}<ErrorView error={state.error} onRetry={refetch} /></>;

  return (
    <>
      {header}
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        keyboardDismissMode="on-drag"
        data={state.data}
        keyExtractor={(m) => m.id}
        renderItem={({ item }) => <MediaCard media={item} onPress={open} layout="wide" showKind />}
        contentContainerStyle={styles.list}
        ItemSeparatorComponent={() => <View style={styles.separator} />}
        onEndReached={loadMore}
        onEndReachedThreshold={0.5}
        ListFooterComponent={loadingMore ? <ActivityIndicator style={styles.footer} accessibilityLabel="Loading more" /> : undefined}
        ListEmptyComponent={
          q ? (
            <EmptyView title={`Nothing matches “${q}”`} detail="Search looks at titles, actors' names, anime series and tags. Check the spelling, or try part of a name." />
          ) : undefined
        }
      />
    </>
  );
}

const fetchTags = (signal: AbortSignal) => listTags({ signal });
// How many tags Search offers before you type: the most used.
const TOP_TAGS = 24;

// BrowseTags is the library's most used tags as chips: one tap lists everything with the tag.
function BrowseTags() {
  const { state } = useAsync(fetchTags);
  if (state.status !== "ok" || state.data.length === 0) return null;
  const top = [...state.data].sort((a, b) => b.videoCount - a.videoCount || a.name.localeCompare(b.name)).slice(0, TOP_TAGS);
  return (
    <View style={styles.tags}>
      <Text variant="title" accessibilityRole="header">
        Browse by tag
      </Text>
      <View style={styles.tagCloud}>
        {top.map((t) => (
          <TagChip key={t.name} name={t.name} count={t.videoCount} />
        ))}
      </View>
    </View>
  );
}

// History is what Search shows before you type: recent searches, newest first, each one tap
// from running again, each removable, and all of them clearable at once; then the library's tags.
function History({ onRun }: { onRun: (q: string) => void }) {
  const { colors } = useAppTheme();
  const history = useSearchHistory();
  if (history.length === 0) {
    return (
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        data={[]}
        renderItem={() => null}
        contentContainerStyle={styles.history}
        ListHeaderComponent={<BrowseTags />}
        ListEmptyComponent={
          <View style={styles.emptyBox}>
            <EmptyView title="Search your library" detail="Find a film, an episode, a comic, an actor or a tag by any part of its name." />
          </View>
        }
      />
    );
  }
  return (
    <FlatList
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="on-drag"
      data={history}
      keyExtractor={(h) => h}
      contentContainerStyle={styles.history}
      ListHeaderComponent={
        <View style={styles.historyHeader}>
          <Text variant="title" accessibilityRole="header">
            Recent searches
          </Text>
          <Pressable onPress={clearHistory} hitSlop={10} accessibilityRole="button" accessibilityLabel="Clear recent searches">
            <Text variant="label" color={colors.primaryText}>
              Clear
            </Text>
          </Pressable>
        </View>
      }
      renderItem={({ item }) => (
        <View style={[styles.entry, { borderBottomColor: colors.border }]}>
          <Pressable
            onPress={() => onRun(item)}
            accessibilityRole="button"
            accessibilityLabel={`Search for ${item}`}
            style={({ pressed }) => [styles.entryMain, pressed && styles.pressed]}
          >
            <Icon sf="clock.arrow.circlepath" md="history" size={18} color={colors.textMuted} />
            <Text numberOfLines={1} style={styles.entryText}>
              {item}
            </Text>
          </Pressable>
          <Pressable
            onPress={() => forget(item)}
            hitSlop={6}
            accessibilityRole="button"
            accessibilityLabel={`Remove ${item} from recent searches`}
            style={({ pressed }) => [styles.remove, pressed && styles.pressed]}
          >
            <Icon sf="xmark" md="close" size={14} color={colors.textMuted} />
          </Pressable>
        </View>
      )}
      ListFooterComponent={<BrowseTags />}
    />
  );
}

const styles = StyleSheet.create({
  list: { paddingHorizontal: space.lg, paddingBottom: space.xxxl, flexGrow: 1 },
  separator: { height: space.xl },
  footer: { margin: space.lg },
  history: { paddingHorizontal: space.lg, paddingBottom: space.xxxl },
  historyHeader: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingVertical: space.md },
  entry: { flexDirection: "row", alignItems: "center", borderBottomWidth: StyleSheet.hairlineWidth },
  entryMain: { flex: 1, flexDirection: "row", alignItems: "center", gap: space.md, minHeight: touch.min + 6 },
  entryText: { flex: 1 },
  remove: { width: touch.min, height: touch.min, alignItems: "flex-end", justifyContent: "center" },
  pressed: { opacity: 0.6 },
  tags: { gap: space.md, paddingTop: space.xl },
  tagCloud: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  emptyBox: { minHeight: 320 },
});
