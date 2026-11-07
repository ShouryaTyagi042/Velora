import { router, Stack, useIsFocused, useLocalSearchParams } from "expo-router";
import { useCallback, useRef, useState, type ComponentRef } from "react";
import { Alert, Pressable, ScrollView, StyleSheet, TextInput, View } from "react-native";

import { ApiError } from "@/api/client";
import {
  addActor,
  addTag,
  deleteForever,
  getMedia,
  listMedia,
  MediaKind,
  moveToBin,
  removeActor,
  removeTag,
  restoreFromBin,
  Section,
  setFavourite,
  type Media,
} from "@/api/media";
import { ActionButton } from "@/components/ActionButton";
import { ActorChip } from "@/components/ActorChip";
import { Button } from "@/components/Button";
import { EpisodeRow } from "@/components/EpisodeRow";
import { ErrorView } from "@/components/ErrorView";
import { Gradient } from "@/components/Gradient";
import { Icon } from "@/components/Icon";
import { MediaCard } from "@/components/MediaCard";
import { Poster } from "@/components/Poster";
import { Rail } from "@/components/Rail";
import { Skeleton } from "@/components/Skeleton";
import { TagChip } from "@/components/TagChip";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { libraryChanged } from "@/hooks/useLibraryChanges";
import { formatBytes, formatDate, formatDuration, formatResolution, formatViews } from "@/lib/format";
import { openMedia } from "@/navigation/openMedia";
import { VideoTeaser } from "@/preview/LoopingClip";
import { resumeAt, resumePage } from "@/progress/resume";
import { gradients, onMedia, radius, space, touch, typography, useAppTheme } from "@/theme";

const MORE = 10;

// Details loads the item and, alongside it, what to show next to it: the rest of its series, or
// more with its first-named actor.
export default function Details() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback(
    async (signal: AbortSignal) => {
      const media = await getMedia(id, { signal });
      const actor = media.actors?.[0]?.name;
      const related = media.series
        ? await listMedia({ kind: MediaKind.Video, series: media.series, limit: 50, signal })
        : actor
          ? await listMedia({ kind: MediaKind.Video, actor, sort: "recent", limit: MORE + 1, signal })
          : { items: [] };
      return { media, related: related.items };
    },
    [id],
  );
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <DetailsSkeleton />;
  // A bad id comes back as a 404 ApiError; ErrorView says "Not found".
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  return <MediaDetails media={state.data.media} related={state.data.related} />;
}

function MediaDetails({ media: initial, related }: { media: Media; related: Media[] }) {
  const { colors } = useAppTheme();
  // The screen's own copy: adding or removing an actor replaces it with the server's answer.
  const [media, setMedia] = useState(initial);
  const isVideo = media.kind === MediaKind.Video;
  const focused = useIsFocused();
  const actors = media.actors ?? []; // omitted from the JSON when there are none
  const open = () => router.push({ pathname: isVideo ? "/player/[id]" : "/reader/[id]", params: { id: media.id } });

  // The main action says where it will take you.
  const at = resumeAt(media);
  const page = resumePage(media);
  const action = isVideo ? (at > 0 ? `Resume from ${formatDuration(at)}` : "Play") : page > 0 ? `Continue on page ${page + 1}` : "Read";
  const watched = media.progress && media.video && !media.progress.completed ? (media.progress.positionMs ?? 0) / media.video.durationMs : null;
  const tags = media.tags ?? [];
  const tagInput = useRef<ComponentRef<typeof TextInput>>(null);
  const scroll = useRef<ComponentRef<typeof ScrollView>>(null);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // run sends one change and puts the server's answer on screen. A failure leaves the screen as it
  // was and says so under the buttons.
  const run = (change: () => Promise<Media>) => {
    if (busy) return;
    setBusy(true);
    setActionError(null);
    change().then(
      (m) => {
        setMedia(m);
        setBusy(false);
        libraryChanged();
      },
      (err: unknown) => {
        setActionError(err instanceof ApiError ? err.message : "Couldn't reach the server. Check your Wi-Fi and try again.");
        setBusy(false);
      },
    );
  };
  const confirmDelete = () =>
    Alert.alert(
      `Delete “${media.title}” forever?`,
      isVideo ? "The video file is removed from the server. This can't be undone." : "The comic's pages are removed from the server. This can't be undone.",
      [
        { text: "Cancel", style: "cancel" },
        {
          text: "Delete forever",
          style: "destructive",
          onPress: () =>
            deleteForever(media.id).then(
              () => {
                libraryChanged();
                router.back();
              },
              (err: unknown) => setActionError(err instanceof ApiError ? err.message : "Couldn't reach the server. Try again."),
            ),
        },
      ],
    );

  return (
    <ScrollView ref={scroll} contentInsetAdjustmentBehavior="never" contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
      <Stack.Screen options={{ title: "" }} />

      {/* The still, edge to edge under the transparent header, is a second Play button. A video's
          clip plays over it while this page is on top; opening the player stops it. */}
      <Pressable onPress={open} accessibilityRole="button" accessibilityLabel={`${action}: ${media.title}`}>
        <Poster media={media} style={styles.hero} aspectRatio={isVideo ? 16 / 10 : 4 / 5}>
          {isVideo && focused && <VideoTeaser media={media} soundStyle={styles.heroSound} />}
          <Gradient colors={gradients.top} style={styles.heroTop} />
          <Gradient colors={gradients.billboard} style={styles.heroFade} />
          <View style={[styles.playBadge, { backgroundColor: onMedia.scrim }]}>
            {isVideo ? (
              <Icon sf="play.fill" md="play_arrow" size={26} color={onMedia.text} />
            ) : (
              <Icon sf="book.fill" md="menu_book" size={24} color={onMedia.text} />
            )}
          </View>
        </Poster>
      </Pressable>

      <View style={styles.body}>
        <View style={styles.heading}>
          {media.section === Section.Anime && (
            <Text variant="caption" color={colors.onPrimary} style={[styles.tag, { backgroundColor: colors.primary }]}>
              Anime
            </Text>
          )}
          <Text style={typography.display} accessibilityRole="header">
            {media.title}
          </Text>
          <Text variant="label" muted>
            {describeKind(media)}
          </Text>
        </View>

        {media.inBin && (
          <View style={[styles.binBanner, { backgroundColor: colors.binTint, borderColor: colors.dangerFill }]} accessibilityRole="summary">
            <View style={styles.binTitle}>
              <Icon sf="trash.fill" md="delete" size={18} color={colors.danger} />
              <Text variant="label">In the bin{media.binnedAt ? ` since ${formatDate(media.binnedAt)}` : ""}</Text>
            </View>
            <Text variant="caption" muted>
              It&apos;s hidden everywhere else. Restore it to bring it back as it was, or delete it to remove its files from the server.
            </Text>
            <View style={styles.binButtons}>
              <Button
                label="Restore"
                icon={{ sf: "arrow.uturn.backward", md: "restore_from_trash" }}
                variant="secondary"
                disabled={busy}
                onPress={() => run(() => restoreFromBin(media.id))}
                style={styles.half}
              />
              <Button label="Delete forever" icon={{ sf: "trash", md: "delete_forever" }} variant="danger" disabled={busy} onPress={confirmDelete} style={styles.half} />
            </View>
          </View>
        )}

        <View style={styles.actions}>
          <Button label={action} icon={isVideo ? { sf: "play.fill", md: "play_arrow" } : { sf: "book.fill", md: "menu_book" }} variant="play" onPress={open} haptic />
          {watched !== null && watched > 0 && (
            <View style={[styles.track, { backgroundColor: colors.surfaceRaised }]} accessibilityLabel={`${Math.round(watched * 100)}% watched`}>
              <View style={[styles.bar, { width: `${Math.min(1, watched) * 100}%`, backgroundColor: colors.primary }]} />
            </View>
          )}
          <Text variant="caption" muted>
            {formatViews(media.views)} · {formatBytes(media.sizeBytes)} · added {formatDate(media.addedAt)}
          </Text>
        </View>

        <View>
          <View style={[styles.actionRow, { borderColor: colors.border }]}>
            <ActionButton
              label={media.favourite ? "Favourited" : "Favourite"}
              icon={media.favourite ? { sf: "heart.fill", md: "favorite" } : { sf: "heart", md: "favorite_border" }}
              active={media.favourite}
              accessibilityLabel={media.favourite ? "Remove from favourites" : "Add to favourites"}
              disabled={busy}
              onPress={() => run(() => setFavourite(media.id, !media.favourite))}
            />
            {isVideo && (
              <ActionButton
                label="Tag"
                icon={{ sf: "number", md: "tag" }}
                accessibilityLabel="Add a tag"
                onPress={() => {
                  scroll.current?.scrollToEnd({ animated: true });
                  tagInput.current?.focus();
                }}
              />
            )}
            {!media.inBin && (
              <ActionButton
                label="Move to bin"
                icon={{ sf: "trash", md: "delete" }}
                tone="danger"
                disabled={busy}
                onPress={() => run(() => moveToBin(media.id))}
              />
            )}
          </View>
          {actionError && (
            <Text variant="caption" color={colors.danger} accessibilityLiveRegion="polite" style={styles.actionError}>
              {actionError}
            </Text>
          )}
        </View>

        {media.series && (
          <View style={styles.section}>
            <Pressable
              onPress={() => router.push({ pathname: "/series/[name]", params: { name: media.series! } })}
              accessibilityRole="button"
              accessibilityLabel={`${media.series}, all episodes`}
              style={styles.sectionHeading}
            >
              <Text variant="title">{media.series}</Text>
              <Icon sf="chevron.right" md="chevron_right" size={14} color={colors.textMuted} />
            </Pressable>
            <View style={styles.episodes}>
              {around(related, media.id).map((e) => (
                <EpisodeRow
                  key={e.id}
                  episode={e}
                  current={e.id === media.id}
                  onPress={(m) => (m.id === media.id ? open() : router.replace({ pathname: "/media/[id]", params: { id: m.id } }))}
                />
              ))}
            </View>
          </View>
        )}

        {isVideo && (
          <View style={styles.section}>
            <Text variant="title" accessibilityRole="header">
              Cast
            </Text>
            {actors.length > 0 && (
              <View style={styles.chips}>
                {actors.map((tag) => (
                  <ActorChip
                    key={tag.name}
                    tag={tag}
                    onRemove={() =>
                      removeActor(media.id, tag.name).then(
                        () => setMedia((m) => ({ ...m, actors: m.actors?.filter((a) => a.name !== tag.name) })),
                        () => {}, // the chip stays; a failed remove changes nothing
                      )
                    }
                  />
                ))}
              </View>
            )}
            <AddField placeholder="Add someone who's in it" label="Actor name" capitalize="words" submit={(name) => addActor(media.id, name)} onAdded={setMedia} />
          </View>
        )}

        {isVideo && (
          <View style={styles.section}>
            <Text variant="title" accessibilityRole="header">
              Tags
            </Text>
            {tags.length > 0 ? (
              <View style={styles.chips}>
                {tags.map((t) => (
                  <TagChip key={t} name={t} onRemove={() => run(() => removeTag(media.id, t))} />
                ))}
              </View>
            ) : (
              <Text variant="caption" muted>
                Words that say what this is, like “pov” or “vintage”. Search finds them, and each tag lists everything that has it.
              </Text>
            )}
            <AddField
              inputRef={tagInput}
              placeholder="Add a tag"
              label="Tag"
              capitalize="none"
              submit={(name) => addTag(media.id, name)}
              onAdded={setMedia}
            />
          </View>
        )}
      </View>

      {!media.series && actors[0] && (
        <Rail
          title={`More with ${actors[0].name}`}
          items={related.filter((m) => m.id !== media.id).slice(0, MORE)}
          keyOf={(m) => m.id}
          render={(m) => <MediaCard media={m} onPress={openMedia} style={styles.railCard} />}
          seeAll={{ pathname: "/actor/[name]", params: { name: actors[0].name } }}
        />
      )}
    </ScrollView>
  );
}

// around is a window of a series' episodes centred on this one: the one before it and the next
// few, which is what you look for on an episode's page.
function around(episodes: Media[], id: string): Media[] {
  const i = episodes.findIndex((e) => e.id === id);
  if (i < 0) return episodes.slice(0, 5);
  const start = Math.max(0, i - 1);
  return episodes.slice(start, start + 5);
}

// describeKind is the meta line: "1:42:10 · 1080p · h264", "Comic · 24 pages".
function describeKind(m: Media): string {
  if (m.kind === MediaKind.Comic) return `Comic · ${m.comic?.pageCount ?? 0} pages`;
  if (!m.video) return "Video · still processing";
  const v = m.video;
  return `${formatDuration(v.durationMs / 1000)} · ${formatResolution(v.width, v.height)} · ${v.codec}`;
}

type AddFieldProps = {
  placeholder: string;
  label: string;
  capitalize: "none" | "words";
  submit: (name: string) => Promise<Media>;
  onAdded: (m: Media) => void;
  inputRef?: React.RefObject<ComponentRef<typeof TextInput> | null>;
};

// AddField is a one-line form (an actor, a tag): type, submit, and the video comes back with it.
function AddField({ placeholder, label, capitalize, submit: send, onAdded, inputRef }: AddFieldProps) {
  const { colors } = useAppTheme();
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    send(name).then(
      (m) => {
        onAdded(m);
        setName("");
        setBusy(false);
        libraryChanged();
      },
      (err: unknown) => {
        // The server's message is written for people ("name must be non-empty"); show it.
        setError(err instanceof ApiError ? err.message : "Couldn't reach the server. Check your Wi-Fi and try again.");
        setBusy(false);
      },
    );
  };

  return (
    <View style={styles.addActor}>
      <View style={styles.addRow}>
        <TextInput
          ref={inputRef}
          value={name}
          onChangeText={setName}
          onSubmitEditing={submit}
          placeholder={placeholder}
          placeholderTextColor={colors.textMuted}
          keyboardAppearance="dark"
          returnKeyType="done"
          autoCapitalize={capitalize}
          autoCorrect={false}
          accessibilityLabel={label}
          style={[styles.input, { color: colors.text, backgroundColor: colors.surface }]}
        />
        <Button label="Add" onPress={submit} disabled={busy || name.trim() === ""} variant="secondary" />
      </View>
      {error && (
        <Text variant="caption" color={colors.danger} accessibilityLiveRegion="polite">
          {error}
        </Text>
      )}
    </View>
  );
}

// DetailsSkeleton has the screen's shape: still, title, meta line, button.
function DetailsSkeleton() {
  return (
    <View accessibilityLabel="Loading">
      <Skeleton aspectRatio={16 / 10} style={styles.hero} />
      <View style={[styles.body, styles.skeletonBody]}>
        <Skeleton width="60%" height={30} />
        <Skeleton width="80%" height={16} />
        <Skeleton height={touch.min} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  content: { paddingBottom: space.xxxl * 2, gap: space.xl },
  hero: { borderRadius: 0, width: "100%" },
  heroTop: { bottom: "70%" },
  heroFade: { top: "40%" },
  // Above the title block, which overlaps the hero's bottom edge.
  heroSound: { bottom: space.xxl + space.sm, right: space.lg },
  playBadge: { position: "absolute", width: 60, height: 60, borderRadius: 30, alignItems: "center", justifyContent: "center" },
  body: { paddingHorizontal: space.lg, gap: space.xl, marginTop: -space.xxl },
  heading: { gap: space.xs + 2 },
  tag: { alignSelf: "flex-start", fontWeight: "700", paddingHorizontal: space.sm, paddingVertical: 2, borderRadius: radius.sm, overflow: "hidden" },
  actions: { gap: space.sm + 2 },
  track: { height: 3, borderRadius: 2, overflow: "hidden" },
  bar: { height: "100%" },
  section: { gap: space.md },
  sectionHeading: { flexDirection: "row", alignItems: "center", gap: space.sm, alignSelf: "flex-start" },
  episodes: { gap: space.sm },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  addActor: { gap: space.xs },
  addRow: { flexDirection: "row", gap: space.sm },
  input: { ...typography.body, flex: 1, borderRadius: radius.md, paddingHorizontal: space.md, minHeight: touch.min },
  railCard: { width: 168 },
  actionRow: { flexDirection: "row", borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: space.xs },
  actionError: { marginTop: space.sm },
  binBanner: { borderRadius: radius.lg, borderWidth: StyleSheet.hairlineWidth, padding: space.lg, gap: space.sm },
  binTitle: { flexDirection: "row", alignItems: "center", gap: space.sm },
  binButtons: { flexDirection: "row", gap: space.sm, marginTop: space.xs },
  half: { flex: 1 },
  skeletonBody: { marginTop: space.lg },
});
