import { router } from "expo-router";
import { Pressable, StyleSheet, View } from "react-native";

import { ActorSource, type ActorTag } from "@/api/media";
import { radius, space, useAppTheme } from "@/theme";
import { Chip } from "./Chip";
import { Icon } from "./Icon";

type Props = { tag: ActorTag; onRemove?: () => void };

// ActorChip opens the Videos tab filtered to this actor. The router encodes the name (spaces,
// accents, "&") for the URL; never concatenate it. Manual actors get a remove button when
// onRemove is given; folder actors never do: the folder owns them, and the next scan would put
// them back.
export function ActorChip({ tag, onRemove }: Props) {
  const { colors } = useAppTheme();
  const removable = onRemove && tag.source === ActorSource.Manual;
  return (
    <View style={styles.row}>
      <Chip
        label={tag.name}
        accessibilityLabel={`${tag.name}, show their videos`}
        onPress={() => router.push({ pathname: "/videos", params: { actor: tag.name } })}
      />
      {removable && (
        <Pressable
          onPress={onRemove}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Remove ${tag.name} from this video`}
          style={({ pressed }) => [styles.remove, { backgroundColor: colors.surface }, pressed && styles.pressed]}
        >
          <Icon sf="xmark" md="close" size={12} color={colors.textMuted} />
        </Pressable>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: space.xs },
  remove: { width: 28, height: 28, borderRadius: radius.pill, alignItems: "center", justifyContent: "center" },
  pressed: { opacity: 0.6 },
});
