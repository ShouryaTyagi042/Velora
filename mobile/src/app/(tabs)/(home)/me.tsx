import type { AndroidSymbol, SFSymbol } from "expo-symbols";
import { router, useFocusEffect, type Href } from "expo-router";
import { useCallback, useEffect } from "react";
import { ActivityIndicator, Alert, Pressable, ScrollView, StyleSheet, View } from "react-native";

import { listMedia, listTags } from "@/api/media";
import { useAuth } from "@/auth/AuthProvider";
import { Icon } from "@/components/Icon";
import { Text } from "@/components/Text";
import { useAsync } from "@/hooks/useAsync";
import { plural } from "@/lib/format";
import { syncLibrary, useLibrarySync, type SyncState } from "@/sync/librarySync";
import { radius, space, touch, typography, useAppTheme } from "@/theme";

// COUNT_UP_TO is how far the rows count before saying "200+": a number, not a census.
const COUNT_UP_TO = 200;

const fetchCounts = async (signal: AbortSignal) => {
  const [favourites, bin, tags] = await Promise.all([
    listMedia({ favourites: true, limit: COUNT_UP_TO, signal }),
    listMedia({ bin: true, limit: COUNT_UP_TO, signal }),
    listTags({ signal }),
  ]);
  const n = (p: { items: unknown[]; nextCursor: string | null }) => `${p.items.length}${p.nextCursor ? "+" : ""}`;
  return { favourites: n(favourites), bin: n(bin), binEmpty: bin.items.length === 0, tags: String(tags.length) };
};

// You: the signed-in person's corner of the library. Their favourites, the library's tags, the bin,
// and signing out, each one row with a count, the way a phone's settings read.
export default function Me() {
  const { colors } = useAppTheme();
  const auth = useAuth();
  const user = auth.status === "signedIn" ? auth.user : null;
  const { state, refetch } = useAsync(fetchCounts);
  useFocusEffect(useCallback(() => refetch(), [refetch])); // back from the bin: counts changed
  const c = state.status === "ok" ? state.data : null;
  const initial = (user?.username ?? "?").slice(0, 1).toUpperCase();

  const signOut = () =>
    Alert.alert("Sign out?", "Your progress is saved on the server. You'll need your password to sign in again.", [
      { text: "Cancel", style: "cancel" },
      { text: "Sign out", style: "destructive", onPress: () => void auth.signOut() },
    ]);

  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={styles.content}>
      <View style={styles.profile}>
        <View style={[styles.avatar, { backgroundColor: colors.primary }]} accessible={false}>
          <Text style={typography.display} color={colors.onPrimary}>
            {initial}
          </Text>
        </View>
        <View style={styles.who}>
          <Text variant="title">{user?.username ?? "Signed in"}</Text>
          <Text variant="caption" muted>
            Velora on this phone
          </Text>
        </View>
      </View>

      <View style={[styles.group, { backgroundColor: colors.surface }]}>
        <Row
          icon={{ sf: "heart.fill", md: "favorite" }}
          tint={colors.primaryText}
          label="Favourites"
          detail={c?.favourites}
          href="/favourites"
        />
        <Divider />
        <Row icon={{ sf: "number", md: "tag" }} tint={colors.primaryText} label="Tags" detail={c?.tags} href="/tags" />
        <Divider />
        <Row
          icon={{ sf: "trash.fill", md: "delete" }}
          tint={c && !c.binEmpty ? colors.danger : colors.textMuted}
          label="Bin"
          detail={c ? (c.binEmpty ? "Empty" : c.bin) : undefined}
          href="/bin"
        />
      </View>

      <View style={styles.section}>
        <View style={[styles.group, { backgroundColor: colors.surface }]}>
          <SyncRow onSynced={refetch} />
        </View>
        <Text variant="caption" muted style={styles.footnote}>
          Copied new files to the server? Sync to add them. It runs on the server, so you can keep browsing.
        </Text>
      </View>

      <View style={[styles.group, { backgroundColor: colors.surface }]}>
        <Pressable
          onPress={signOut}
          accessibilityRole="button"
          style={({ pressed }) => [styles.row, pressed && { backgroundColor: colors.surfaceRaised }]}
        >
          <Icon sf="rectangle.portrait.and.arrow.right" md="logout" size={20} color={colors.danger} />
          <Text color={colors.danger} style={styles.rowLabel}>
            Sign out
          </Text>
        </Pressable>
      </View>
    </ScrollView>
  );
}

type RowProps = { icon: { sf: SFSymbol; md: AndroidSymbol }; tint: string; label: string; detail?: string; href: Href };

function Row({ icon, tint, label, detail, href }: RowProps) {
  const { colors } = useAppTheme();
  return (
    <Pressable
      onPress={() => router.push(href)}
      accessibilityRole="button"
      accessibilityLabel={detail ? `${label}, ${detail}` : label}
      style={({ pressed }) => [styles.row, pressed && { backgroundColor: colors.surfaceRaised }]}
    >
      <Icon sf={icon.sf} md={icon.md} size={20} color={tint} />
      <Text style={styles.rowLabel}>{label}</Text>
      {detail !== undefined && (
        <Text variant="label" muted>
          {detail}
        </Text>
      )}
      <Icon sf="chevron.right" md="chevron_right" size={14} color={colors.textMuted} />
    </Pressable>
  );
}

// SyncRow starts a library sync and says how it's going: a spinner with the scan's progress
// while it runs, then what it found, or why it failed.
function SyncRow({ onSynced }: { onSynced: () => void }) {
  const { colors } = useAppTheme();
  const sync = useLibrarySync();
  const syncing = sync.status === "syncing";
  const finishedAt = sync.status === "done" || sync.status === "failed" ? sync.at : 0;
  useEffect(() => {
    if (finishedAt) onSynced(); // new files can land in the bin's or favourites' counts
  }, [finishedAt, onSynced]);
  const detail = describeSync(sync);
  return (
    <Pressable
      onPress={() => void syncLibrary()}
      disabled={syncing}
      accessibilityRole="button"
      accessibilityLabel={detail ? `Sync library, ${detail}` : "Sync library"}
      accessibilityState={{ busy: syncing, disabled: syncing }}
      style={({ pressed }) => [styles.row, pressed && { backgroundColor: colors.surfaceRaised }]}
    >
      <Icon sf="arrow.triangle.2.circlepath" md="sync" size={20} color={colors.primaryText} />
      <View style={styles.rowLabel}>
        <Text>{syncing ? "Syncing library…" : "Sync library"}</Text>
        {detail !== "" && (
          <Text variant="caption" muted={sync.status !== "failed"} color={sync.status === "failed" ? colors.danger : undefined} numberOfLines={2}>
            {detail}
          </Text>
        )}
      </View>
      {syncing && <ActivityIndicator color={colors.textMuted} />}
    </Pressable>
  );
}

function describeSync(s: SyncState): string {
  switch (s.status) {
    case "idle":
      return "";
    case "syncing":
      return s.total ? `${s.done ?? 0} of ${s.total} files` : "Looking for new and removed files";
    case "failed":
      return `Didn't finish: ${s.error}`;
    case "done": {
      const at = new Date(s.at).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
      if (!s.result) return `Synced at ${at}`;
      const gone = s.result.missing > 0 ? `, ${s.result.missing} no longer on disk` : "";
      return `${plural(s.result.found, "item")} in the library${gone}. Synced at ${at}`;
    }
  }
}

function Divider() {
  const { colors } = useAppTheme();
  return <View style={[styles.divider, { backgroundColor: colors.border }]} />;
}

const styles = StyleSheet.create({
  content: { padding: space.lg, gap: space.xl },
  profile: { flexDirection: "row", alignItems: "center", gap: space.lg },
  avatar: { width: 64, height: 64, borderRadius: radius.lg, alignItems: "center", justifyContent: "center" },
  who: { gap: 2 },
  group: { borderRadius: radius.lg, overflow: "hidden" },
  row: { flexDirection: "row", alignItems: "center", gap: space.md, minHeight: touch.min + 8, paddingHorizontal: space.lg },
  rowLabel: { flex: 1, gap: 2, paddingVertical: space.sm },
  section: { gap: space.sm },
  footnote: { paddingHorizontal: space.lg },
  divider: { height: StyleSheet.hairlineWidth, marginLeft: space.lg + 20 + space.md },
});
