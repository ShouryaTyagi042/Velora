import { router } from "expo-router";
import { Pressable, StyleSheet, View } from "react-native";

import type { MediaType } from "@/lib/mediaType";
import { radius, space, useAppTheme } from "@/theme";
import { Icon } from "./Icon";
import { Text } from "./Text";

// type carries a list's Videos / Anime choice through to the tag's page.
type Props = { name: string; count?: number; type?: MediaType; onRemove?: () => void };

// TagChip is a tag: a hash and the word, in a soft blue pill, so tags read as a different thing
// from actors and filters at a glance. Tapping it lists every video with the tag; onRemove adds a
// small x that untags this video.
export function TagChip({ name, count, type, onRemove }: Props) {
  const { colors } = useAppTheme();
  return (
    <View style={[styles.chip, { backgroundColor: colors.surfaceRaised, borderColor: colors.border }]}>
      <Pressable
        onPress={() => router.push({ pathname: "/tag/[name]", params: type && type !== "all" ? { name, type } : { name } })}
        hitSlop={{ top: 6, bottom: 6 }}
        accessibilityRole="button"
        accessibilityLabel={`Tag ${name}${count !== undefined ? `, ${count} videos` : ""}, show all`}
        style={({ pressed }) => [styles.main, pressed && styles.pressed]}
      >
        <Text variant="label" color={colors.primaryText}>
          #
        </Text>
        <Text variant="label">{name}</Text>
        {count !== undefined && (
          <Text variant="caption" muted>
            {count}
          </Text>
        )}
      </Pressable>
      {onRemove && (
        <Pressable
          onPress={onRemove}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Remove the tag ${name}`}
          style={({ pressed }) => [styles.remove, pressed && styles.pressed]}
        >
          <Icon sf="xmark" md="close" size={11} color={colors.textMuted} />
        </Pressable>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  chip: { flexDirection: "row", alignItems: "center", borderRadius: radius.pill, borderWidth: StyleSheet.hairlineWidth, minHeight: 32 },
  main: { flexDirection: "row", alignItems: "baseline", gap: 3, paddingLeft: space.md, paddingRight: space.md, paddingVertical: space.xs + 2 },
  remove: { paddingRight: space.md - 2, paddingLeft: 0, height: 32, justifyContent: "center" },
  pressed: { opacity: 0.6 },
});
