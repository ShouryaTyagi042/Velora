import { router, Stack, useIsFocused, useLocalSearchParams } from "expo-router";
import { useCallback, useRef, useState, type ComponentRef, type ReactNode } from "react";
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
  randomMedia,
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
import { formatDate, formatDuration, formatResolution, formatRuntime, plural } from "@/lib/format";
import { openMedia } from "@/navigation/openMedia";
import { VideoTeaser } from "@/preview/LoopingClip";
import { resumeAt, resumePage } from "@/progress/resume";
import { gradients, onMedia, radius, space, touch, typography, useAppTheme } from "@/theme";

const MORE = 10;
// PICKS: how many random videos More to watch shows.
const PICKS = 5;
// NEW_FOR: how long something counts as newly added, in ms.
const NEW_FOR = 7 * 24 * 3600 * 1000;

// Details loads the item and, alongside it, what to show next to it: the rest of its series, or
// more with its first-named actor.
export default function Details() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback(
    async (signal: AbortSignal) => {
      const media = await getMedia(id, { signal });
      const actor = media.actors?.[0]?.name;
      const isVideo = media.kind === MediaKind.Video;
      const [related, picks] = await Promise.all([
        media.series
          ? listMedia({ kind: MediaKind.Video, series: media.series, limit: 50, signal })
          : actor
            ? listMedia({ kind: MediaKind.Video, actor, sort: "recent", limit: MORE + 1, signal })
            : { items: [] },
        // More to watch: a few at random from the same shelf (films or anime). One extra, in case
        // this one is drawn.
        isVideo ? randomMedia(PICKS + 1, { kind: MediaKind.Video, section: media.section, signal }) : [],
      ]);
      return { media, related: related.items, picks: picks.filter((m) => m.id !== media.id).slice(0, PICKS) };
    },
    [id],
  );
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <DetailsSkeleton />;
  // A bad id comes back as a 404 ApiError; ErrorView says "Not found".
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  return <MediaDetails media={state.data.media} related={state.data.related} picks={state.data.picks} />;
}

// The page reads top to bottom the way you decide: what it is (the clip playing, the title, a line
// of facts), the one thing to do (Play), the quick things (Favourite, Tag, Bin), then
// what's around it (episodes, cast, tags), and more to watch.
function MediaDetails({ media: initial, related, picks }: { media: Media; related: Media[]; picks: Media[] }) {
  const { colors } = useAppTheme();
  // The screen's own copy: every change replaces it with the server's answer.
  const [media, setMedia] = useState(initial);
  const isVideo = media.kind === MediaKind.Video;
  const focused = useIsFocused();
  const actors = media.actors ?? []; // omitted from the JSON when there are none
  const tags = media.tags ?? [];
  const hasClip = !!(media.video?.short || media.video?.preview);
  const open = (fromStart = false) =>
    router.push({ pathname: isVideo ? "/player/[id]" : "/reader/[id]", params: fromStart ? { id: media.id, from: "start" } : { id: media.id } });

  // The main action says where it will take you.
  const at = resumeAt(media);
  const page = resumePage(media);
  const resuming = isVideo ? at > 0 : page > 0;
  const action = isVideo ? (resuming ? `Resume from ${formatDuration(at)}` : "Play") : resuming ? `Continue on page ${page + 1}` : "Read";
  const progress = progressOf(media, at, page);

  const scroll = useRef<ComponentRef<typeof ScrollView>>(null);
  const tagsY = useRef(0);
  const [addingTag, setAddingTag] = useState(false);
  const [addingActor, setAddingActor] = useState(false);
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
  const startTagging = () => {
    setAddingTag(true);
    scroll.current?.scrollTo({ y: Math.max(0, tagsY.current - space.xxxl * 2), animated: true });
  };

  return (
    <ScrollView ref={scroll} contentInsetAdjustmentBehavior="never" contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
      <Stack.Screen options={{ title: "" }} />

      {/* The artwork, edge to edge under the transparent header, is a second Play button. A video's
          clip plays over it while this page is on top; opening the player stops it. */}
      <Pressable onPress={() => open()} accessibilityRole="button" accessibilityLabel={`${action}: ${media.title}`}>
        <Poster media={media} style={styles.hero} aspectRatio={isVideo ? 16 / 10 : 4 / 5}>
          {isVideo && focused && <VideoTeaser media={media} soundStyle={styles.heroSound} />}
          <Gradient colors={gradients.top} style={styles.heroTop} />
          <Gradient colors={gradients.billboard} style={styles.heroFade} />
          {/* The clip is the invitation; the glyph is only for a still. */}
          {!(isVideo && hasClip) && (
            <View style={[styles.playBadge, { backgroundColor: onMedia.scrim }]}>
              <Icon sf={isVideo ? "play.fill" : "book.fill"} md={isVideo ? "play_arrow" : "menu_book"} size={24} color={onMedia.text} />
            </View>
          )}
        </Poster>
      </Pressable>

      <View style={styles.body}>
        <View style={styles.heading}>
          <Kicker media={media} />
          <Text style={typography.display} accessibilityRole="header">
            {media.title}
          </Text>
          <Facts media={media} />
        </View>

        {media.inBin && (
          <View style={[styles.binBanner, { backgroundColor: colors.binTint, borderColor: colors.dangerFill }]} accessibilityRole="summary">
            <View style={styles.inline}>
              <Icon sf="trash.fill" md="delete" size={18} color={colors.danger} />
              <Text variant="label">In the bin{media.binnedAt ? ` since ${formatDate(media.binnedAt)}` : ""}</Text>
            </View>
            <Text variant="caption" muted>
              It&apos;s hidden everywhere else. Restore it to bring it back as it was, or delete it to remove its files from the server.
            </Text>
            <View style={styles.inline}>
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

        <View style={styles.play}>
          <Button
            label={action}
            icon={isVideo ? { sf: "play.fill", md: "play_arrow" } : { sf: "book.fill", md: "menu_book" }}
            variant="play"
            onPress={() => open()}
            haptic
          />
          {progress && (
            <View style={styles.progress}>
              <View
                style={[styles.track, { backgroundColor: colors.surfaceRaised }]}
                accessible
                accessibilityLabel={`${Math.round(progress.fraction * 100)}% ${isVideo ? "watched" : "read"}, ${progress.left}`}
              >
                <View style={[styles.bar, { width: `${progress.fraction * 100}%`, backgroundColor: colors.primary }]} />
              </View>
              <Text variant="caption" muted>
                {progress.left}
              </Text>
              <Pressable onPress={() => open(true)} hitSlop={10} accessibilityRole="button" accessibilityLabel={`Start ${media.title} over`}>
                <Text variant="caption" color={colors.primaryText} style={styles.startOver}>
                  Start over
                </Text>
              </Pressable>
            </View>
          )}
        </View>

        <View>
          <View style={styles.actionRow}>
            <ActionButton
              label={media.favourite ? "Favourited" : "Favourite"}
              icon={media.favourite ? { sf: "heart.fill", md: "favorite" } : { sf: "heart", md: "favorite_border" }}
              active={media.favourite}
              accessibilityLabel={media.favourite ? "Remove from favourites" : "Add to favourites"}
              disabled={busy}
              onPress={() => run(() => setFavourite(media.id, !media.favourite))}
            />
            {isVideo && <ActionButton label="Tag" icon={{ sf: "number", md: "tag" }} accessibilityLabel="Add a tag" onPress={startTagging} />}
            {!media.inBin && (
              <ActionButton label="Bin" icon={{ sf: "trash", md: "delete" }} tone="danger" accessibilityLabel="Move to bin" disabled={busy} onPress={() => run(() => moveToBin(media.id))} />
            )}
          </View>
          {actionError && (
            <Text variant="caption" color={colors.danger} accessibilityLiveRegion="polite" style={styles.actionError}>
              {actionError}
            </Text>
          )}
        </View>

        {media.series && (
          <Block
            title="Episodes"
            subtitle={media.series}
            action={{
              label: `All ${related.length}`,
              onPress: () => router.push({ pathname: "/series/[name]", params: { name: media.series! } }),
            }}
          >
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
          </Block>
        )}

        {isVideo && (
          <Block title="Cast">
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
              {!addingActor && <AddChip label={actors.length ? "Add" : "Add someone"} onPress={() => setAddingActor(true)} />}
            </View>
            {addingActor && (
              <AddField
                placeholder="Who's in it?"
                label="Actor name"
                capitalize="words"
                submit={(name) => addActor(media.id, name)}
                onAdded={setMedia}
                onClose={() => setAddingActor(false)}
              />
            )}
          </Block>
        )}

        {isVideo && (
          <View onLayout={(e) => (tagsY.current = e.nativeEvent.layout.y)}>
            <Block title="Tags">
              <View style={styles.chips}>
                {tags.map((t) => (
                  <TagChip key={t} name={t} onRemove={() => run(() => removeTag(media.id, t))} />
                ))}
                {!addingTag && <AddChip label={tags.length ? "Add" : "Add a tag"} onPress={() => setAddingTag(true)} />}
              </View>
              {addingTag ? (
                <AddField
                  placeholder="pov, vintage, favourite scene…"
                  label="Tag"
                  capitalize="none"
                  submit={(name) => addTag(media.id, name)}
                  onAdded={setMedia}
                  onClose={() => setAddingTag(false)}
                />
              ) : (
                tags.length === 0 && (
                  <Text variant="caption" muted>
                    A word or two that says what this is. Search finds it, and each tag lists everything that has it.
                  </Text>
                )
              )}
            </Block>
          </View>
        )}

      </View>

      {!media.series && actors[0] && (
        <Rail
          title={`More with ${actors[0].name}`}
          items={related.filter((m) => m.id !== media.id).slice(0, MORE)}
          keyOf={(m) => m.id}
          render={(m) => <MediaCard media={m} onPress={openMedia} />}
          seeAll={{ pathname: "/actor/[name]", params: { name: actors[0].name } }}
        />
      )}

      {picks.length > 0 && (
        <Rail title="More to watch" items={picks} keyOf={(m) => m.id} render={(m) => <MediaCard media={m} onPress={openMedia} />} />
      )}
    </ScrollView>
  );
}

// Kicker is the line over the title that says what shelf this is from: Anime and its series (a
// link to every episode), a film's cast, or Comic.
function Kicker({ media }: { media: Media }) {
  const { colors } = useAppTheme();
  const anime = media.section === Section.Anime;
  const label = media.kind === MediaKind.Comic ? "Comic" : anime ? "Anime" : "Film";
  const who = media.series ?? media.actors?.map((a) => a.name).join(", ");
  return (
    <View style={styles.inline}>
      <Text variant="caption" color={colors.onPrimary} style={[styles.kind, { backgroundColor: anime || media.kind === MediaKind.Comic ? colors.primary : colors.secondaryFill }]}>
        {label}
      </Text>
      {who ? (
        <Text variant="label" color={media.series ? colors.primaryText : colors.textMuted} numberOfLines={1} style={styles.flex}>
          {who}
        </Text>
      ) : null}
    </View>
  );
}

// Facts is the line under the title: how long, how sharp, how watched, how new. Separate small
// pieces, the quality in an outlined box, so it reads at a glance instead of as a sentence.
function Facts({ media }: { media: Media }) {
  const { colors } = useAppTheme();
  const v = media.video;
  const [openedAt] = useState(Date.now); // read once: "New" doesn't flip while you look
  const isNew = openedAt - Date.parse(media.addedAt) < NEW_FOR;
  return (
    <View style={styles.facts}>
      {isNew && (
        <Text variant="label" color={colors.primaryText}>
          New
        </Text>
      )}
      {v ? (
        <>
          <Text variant="label" muted>
            {formatRuntime(v.durationMs / 1000)}
          </Text>
          <Text variant="caption" muted style={[styles.quality, { borderColor: colors.chipBorder }]}>
            {formatResolution(v.width, v.height)}
          </Text>
        </>
      ) : media.comic ? (
        <Text variant="label" muted>
          {plural(media.comic.pageCount, "page")}
        </Text>
      ) : (
        <Text variant="label" muted>
          Still processing
        </Text>
      )}
      {media.views > 0 && (
        <View style={styles.inlineTight}>
          <Icon sf="eye" md="visibility" size={13} color={colors.textMuted} />
          <Text variant="label" muted>
            {media.views}
          </Text>
        </View>
      )}
    </View>
  );
}

// progressOf is how far you are, for the bar under Play, with what's left in words.
function progressOf(m: Media, at: number, page: number): { fraction: number; left: string } | null {
  if (m.video && at > 0) {
    const total = m.video.durationMs / 1000;
    return { fraction: Math.min(1, at / total), left: `${formatRuntime(total - at)} left` };
  }
  if (m.comic && page > 0) {
    const count = m.comic.pageCount;
    return { fraction: Math.min(1, (page + 1) / count), left: `${plural(count - page - 1, "page")} left` };
  }
  return null;
}

// Block is a titled block of the page, with an optional link at the right of its heading.
function Block({ title, subtitle, action, children }: { title: string; subtitle?: string; action?: { label: string; onPress: () => void }; children: ReactNode }) {
  const { colors } = useAppTheme();
  return (
    <View style={styles.section}>
      <View style={styles.sectionHeading}>
        <View style={styles.flex}>
          <Text variant="title" accessibilityRole="header">
            {title}
          </Text>
          {subtitle && (
            <Text variant="caption" muted numberOfLines={1}>
              {subtitle}
            </Text>
          )}
        </View>
        {action && (
          <Pressable onPress={action.onPress} hitSlop={10} accessibilityRole="button" style={({ pressed }) => [styles.inlineTight, pressed && styles.pressed]}>
            <Text variant="label" color={colors.primaryText}>
              {action.label}
            </Text>
            <Icon sf="chevron.right" md="chevron_right" size={12} color={colors.primaryText} />
          </Pressable>
        )}
      </View>
      {children}
    </View>
  );
}

// AddChip sits at the end of a row of chips: the way to add one more, drawn dashed so it reads as
// a slot to fill rather than another name.
function AddChip({ label, onPress }: { label: string; onPress: () => void }) {
  const { colors } = useAppTheme();
  return (
    <Pressable
      onPress={onPress}
      hitSlop={{ top: 6, bottom: 6 }}
      accessibilityRole="button"
      style={({ pressed }) => [styles.addChip, { borderColor: colors.chipBorder }, pressed && styles.pressed]}
    >
      <Icon sf="plus" md="add" size={12} color={colors.primaryText} />
      <Text variant="label" color={colors.primaryText}>
        {label}
      </Text>
    </Pressable>
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

type AddFieldProps = {
  placeholder: string;
  label: string;
  capitalize: "none" | "words";
  submit: (name: string) => Promise<Media>;
  onAdded: (m: Media) => void;
  onClose: () => void;
};

// AddField is the one-line form an Add chip opens (an actor, a tag): type, return, and the video
// comes back with it; the field stays open for the next one. Done, or leaving it empty, closes it.
function AddField({ placeholder, label, capitalize, submit: send, onAdded, onClose }: AddFieldProps) {
  const { colors } = useAppTheme();
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = () => {
    if (busy) return;
    if (name.trim() === "") return onClose();
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
    <View style={styles.addField}>
      <View style={[styles.addRow, { backgroundColor: colors.surface, borderColor: colors.border }]}>
        <TextInput
          autoFocus
          value={name}
          onChangeText={setName}
          onSubmitEditing={submit}
          onBlur={() => name.trim() === "" && onClose()}
          submitBehavior="submit"
          placeholder={placeholder}
          placeholderTextColor={colors.textMuted}
          keyboardAppearance="dark"
          returnKeyType="done"
          autoCapitalize={capitalize}
          autoCorrect={false}
          accessibilityLabel={label}
          style={[styles.input, { color: colors.text }]}
        />
        <Pressable
          onPress={name.trim() ? submit : onClose}
          disabled={busy}
          hitSlop={6}
          accessibilityRole="button"
          accessibilityLabel={name.trim() ? `Add ${name.trim()}` : "Done"}
          style={({ pressed }) => [styles.addButton, { backgroundColor: name.trim() ? colors.primary : colors.surfaceRaised }, (pressed || busy) && styles.pressed]}
        >
          <Text variant="label" color={name.trim() ? colors.onPrimary : colors.text}>
            {name.trim() ? "Add" : "Done"}
          </Text>
        </Pressable>
      </View>
      {error && (
        <Text variant="caption" color={colors.danger} accessibilityLiveRegion="polite">
          {error}
        </Text>
      )}
    </View>
  );
}

// DetailsSkeleton has the screen's shape: artwork, title, facts, button, actions.
function DetailsSkeleton() {
  return (
    <View accessibilityLabel="Loading">
      <Skeleton aspectRatio={16 / 10} style={styles.hero} />
      <View style={[styles.body, styles.skeletonBody]}>
        <Skeleton width="30%" height={16} />
        <Skeleton width="70%" height={30} />
        <Skeleton width="45%" height={16} />
        <Skeleton height={touch.min} />
        <Skeleton height={touch.min + 12} />
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
  playBadge: { position: "absolute", width: 56, height: 56, borderRadius: 28, alignItems: "center", justifyContent: "center" },
  body: { paddingHorizontal: space.lg, gap: space.xl, marginTop: -space.xxl },
  heading: { gap: space.sm },
  kind: { fontWeight: "700", paddingHorizontal: space.sm, paddingVertical: 2, borderRadius: radius.sm, overflow: "hidden" },
  inline: { flexDirection: "row", alignItems: "center", gap: space.sm },
  inlineTight: { flexDirection: "row", alignItems: "center", gap: space.xs },
  flex: { flex: 1 },
  facts: { flexDirection: "row", alignItems: "center", flexWrap: "wrap", columnGap: space.md, rowGap: space.xs },
  quality: { borderWidth: 1, borderRadius: radius.sm, paddingHorizontal: space.xs + 1, fontWeight: "700", overflow: "hidden" },
  play: { gap: space.md },
  progress: { flexDirection: "row", alignItems: "center", gap: space.md },
  track: { flex: 1, height: 4, borderRadius: 2, overflow: "hidden" },
  bar: { height: "100%" },
  startOver: { fontWeight: "700" },
  actionRow: { flexDirection: "row", marginHorizontal: -space.sm },
  actionError: { marginTop: space.sm },
  binBanner: { borderRadius: radius.lg, borderWidth: StyleSheet.hairlineWidth, padding: space.lg, gap: space.sm },
  half: { flex: 1 },
  section: { gap: space.md },
  sectionHeading: { flexDirection: "row", alignItems: "flex-end", gap: space.md },
  episodes: { gap: space.sm },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: space.sm, alignItems: "center" },
  addChip: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.xs,
    minHeight: 32,
    paddingHorizontal: space.md,
    borderRadius: radius.pill,
    borderWidth: 1,
    borderStyle: "dashed",
  },
  addField: { gap: space.xs },
  addRow: { flexDirection: "row", alignItems: "center", borderRadius: radius.lg, borderWidth: StyleSheet.hairlineWidth, paddingLeft: space.md, paddingRight: space.xs },
  input: { ...typography.body, flex: 1, minHeight: touch.min },
  addButton: { borderRadius: radius.md, paddingHorizontal: space.md, minHeight: 34, justifyContent: "center" },
  pressed: { opacity: 0.6 },
  skeletonBody: { marginTop: space.lg, gap: space.md },
});
