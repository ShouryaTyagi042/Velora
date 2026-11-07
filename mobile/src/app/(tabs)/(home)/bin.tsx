import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, Alert, FlatList, Pressable, StyleSheet, View } from "react-native";

import { ApiError } from "@/api/client";
import { deleteForever, emptyBin, listMedia, MediaKind, restoreFromBin, type Media } from "@/api/media";
import { Button } from "@/components/Button";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Icon } from "@/components/Icon";
import { Poster } from "@/components/Poster";
import { Skeleton } from "@/components/Skeleton";
import { Text } from "@/components/Text";
import { TypeFilter } from "@/components/TypeFilter";
import { libraryChanged, useRefetchOnLibraryChange } from "@/hooks/useLibraryChanges";
import { usePages } from "@/hooks/usePages";
import { formatAgo, plural } from "@/lib/format";
import { parseMediaType, typeFilter, typeNoun } from "@/lib/mediaType";
import { radius, space, touch, useAppTheme } from "@/theme";

const failure = (err: unknown) => (err instanceof ApiError ? err.message : "Couldn't reach the server. Check your Wi-Fi and try again.");

// The bin: what's been moved out of the library, most recent first. Nothing here is gone yet:
// each item can be restored, exactly as it was, or deleted for good (its files removed from the
// server). Empty bin deletes everything here. Both deletes ask first; nothing else does.
export default function Bin() {
  const { colors } = useAppTheme();
  // ?type=videos|anime|comics shows (and empties) only that part of the bin.
  const type = parseMediaType(useLocalSearchParams<{ type?: string }>().type);
  const all = type === "all";
  const fetchBin = useCallback(
    (after: string | undefined, signal: AbortSignal) => listMedia({ ...typeFilter(type), bin: true, sort: "binned", after, signal }),
    [type],
  );
  const { state, refetch, loadMore, loadingMore } = usePages(fetchBin);
  useRefetchOnLibraryChange(refetch);
  const [working, setWorking] = useState<string | null>(null); // an item id, or "all"
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null);
  const items = state.status === "ok" ? state.data : [];

  const act = useCallback(
    (key: string, change: () => Promise<string>) => {
      setWorking(key);
      setMessage(null);
      change().then(
        (text) => {
          setWorking(null);
          setMessage({ text, error: false });
          libraryChanged();
          refetch();
        },
        (err: unknown) => {
          setWorking(null);
          setMessage({ text: failure(err), error: true });
        },
      );
    },
    [refetch],
  );

  const restore = (m: Media) => act(m.id, () => restoreFromBin(m.id).then(() => `Restored “${m.title}”.`));
  const remove = (m: Media) =>
    Alert.alert(`Delete “${m.title}” forever?`, "Its files are removed from the server. This can't be undone.", [
      { text: "Cancel", style: "cancel" },
      { text: "Delete forever", style: "destructive", onPress: () => act(m.id, () => deleteForever(m.id).then(() => `Deleted “${m.title}”.`)) },
    ]);
  const emptyLabel = all ? "Empty bin" : `Delete all ${typeNoun(type)}`;
  const empty = () =>
    Alert.alert(
      all ? "Empty the bin?" : `Delete every ${typeNoun(type) === "anime" ? "anime episode" : typeNoun(type).replace(/s$/, "")} in the bin?`,
      `${state.status === "ok" && items.length < 50 ? plural(items.length, "item") : all ? "Everything in the bin" : `All the ${typeNoun(type)} in the bin`} will be deleted for good, files and all. ${all ? "" : "The rest of the bin stays. "}This can't be undone.`,
      [
        { text: "Cancel", style: "cancel" },
        {
          text: emptyLabel,
          style: "destructive",
          onPress: () =>
            act("all", () =>
              emptyBin(typeFilter(type)).then((r) =>
                r.failed > 0
                  ? `Deleted ${plural(r.deleted, "item")}. ${plural(r.failed, "item")} couldn't be removed and stayed in the bin; the server's log says why.`
                  : `Deleted ${plural(r.deleted, "item")}.`,
              ),
            ),
        },
      ],
    );

  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;

  return (
    <FlatList
      contentInsetAdjustmentBehavior="automatic"
      data={items}
      keyExtractor={(m) => m.id}
      contentContainerStyle={styles.list}
      ItemSeparatorComponent={() => <View style={[styles.separator, { backgroundColor: colors.border }]} />}
      onEndReached={loadMore}
      onEndReachedThreshold={0.6}
      ListHeaderComponent={
        <View style={styles.header}>
          <TypeFilter value={type} />
          {items.length > 0 && (
            <>
              <Text muted>Items here are hidden everywhere else. Restore one to put it back as it was.</Text>
              <Button
                label={working === "all" ? "Deleting…" : emptyLabel}
                icon={{ sf: "trash", md: "delete_forever" }}
                variant="danger"
                disabled={working !== null}
                onPress={empty}
              />
            </>
          )}
          {message && (
            <Text variant="caption" color={message.error ? colors.danger : colors.textMuted} accessibilityLiveRegion="polite">
              {message.text}
            </Text>
          )}
        </View>
      }
      renderItem={({ item }) => <BinRow media={item} busy={working === item.id || working === "all"} onRestore={restore} onDelete={remove} />}
      ListFooterComponent={loadingMore ? <ActivityIndicator style={styles.footer} /> : undefined}
      ListEmptyComponent={
        state.status === "loading" ? (
          <View style={styles.skeletons} accessibilityLabel="Loading">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} height={84} />
            ))}
          </View>
        ) : (
          <EmptyView
            title={all ? "The bin is empty" : `No ${typeNoun(type)} in the bin`}
            detail="Things you move to the bin from their details page wait here until you restore or delete them."
          />
        )
      }
    />
  );
}

type RowProps = { media: Media; busy: boolean; onRestore: (m: Media) => void; onDelete: (m: Media) => void };

// BinRow: the item's picture (tap it for its details), what it is and when it was binned, and the
// two ways out of the bin side by side, the irreversible one in red.
function BinRow({ media, busy, onRestore, onDelete }: RowProps) {
  const { colors } = useAppTheme();
  const isVideo = media.kind === MediaKind.Video;
  return (
    <View style={[styles.row, busy && styles.busy]}>
      <Pressable
        onPress={() => router.push({ pathname: "/media/[id]", params: { id: media.id } })}
        accessibilityRole="button"
        accessibilityLabel={`${media.title}, details`}
        style={styles.thumbBox}
      >
        <Poster media={media} style={isVideo ? styles.still : styles.cover} />
      </Pressable>
      <View style={styles.text}>
        <Text variant="label" numberOfLines={2}>
          {media.title}
        </Text>
        <Text variant="caption" muted>
          {isVideo ? (media.section === "anime" ? "Anime" : "Video") : "Comic"}
          {media.binnedAt ? `  ·  binned ${formatAgo(media.binnedAt)}` : ""}
        </Text>
        <View style={styles.buttons}>
          <Pressable
            onPress={() => onRestore(media)}
            disabled={busy}
            hitSlop={{ top: 5, bottom: 5 }}
            accessibilityRole="button"
            accessibilityLabel={`Restore ${media.title}`}
            style={({ pressed }) => [styles.small, { backgroundColor: colors.surfaceRaised }, pressed && styles.pressed]}
          >
            <Icon sf="arrow.uturn.backward" md="restore_from_trash" size={14} color={colors.text} />
            <Text variant="caption">Restore</Text>
          </Pressable>
          <Pressable
            onPress={() => onDelete(media)}
            disabled={busy}
            hitSlop={{ top: 5, bottom: 5 }}
            accessibilityRole="button"
            accessibilityLabel={`Delete ${media.title} forever`}
            style={({ pressed }) => [styles.small, { backgroundColor: colors.binTint }, pressed && styles.pressed]}
          >
            <Icon sf="trash" md="delete_forever" size={14} color={colors.danger} />
            <Text variant="caption" color={colors.danger}>
              Delete
            </Text>
          </Pressable>
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  list: { paddingHorizontal: space.lg, paddingBottom: space.xxxl, flexGrow: 1 },
  header: { gap: space.md, marginBottom: space.lg },
  separator: { height: StyleSheet.hairlineWidth, marginVertical: space.md },
  row: { flexDirection: "row", gap: space.md, alignItems: "center" },
  busy: { opacity: 0.45 },
  thumbBox: { width: "38%", alignItems: "center" },
  still: { width: "100%", borderRadius: radius.md },
  cover: { width: "55%", borderRadius: radius.md },
  text: { flex: 1, gap: space.xs },
  buttons: { flexDirection: "row", gap: space.sm, marginTop: space.xs },
  small: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.xs,
    minHeight: touch.min - 10,
    paddingHorizontal: space.md,
    borderRadius: radius.pill,
  },
  pressed: { opacity: 0.6 },
  footer: { margin: space.lg },
  skeletons: { gap: space.lg },
});
