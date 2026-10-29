import { router } from "expo-router";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ActorSource, type ActorTag } from "@/api/media";
import { Chip } from "./Chip";

type Props = { tag: ActorTag; onRemove?: () => void };

// ActorChip opens the Library filtered to this actor. push, not navigate, so back returns
// here. The router encodes the name (spaces, accents, "&") for the URL; never concatenate it.
// Manual actors get a remove button when onRemove is given; folder actors never do: the folder
// owns them, and the next scan would put them back.
export function ActorChip({ tag, onRemove }: Props) {
  const removable = onRemove && tag.source === ActorSource.Manual;
  return (
    <View style={styles.row}>
      <Chip label={tag.name} onPress={() => router.push({ pathname: "/library", params: { actor: tag.name } })} />
      {removable && (
        <Pressable
          onPress={onRemove}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Remove ${tag.name} from this video`}
          style={({ pressed }) => [styles.remove, pressed && styles.pressed]}
        >
          <Text style={styles.removeText}>✕</Text>
        </Pressable>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 4 },
  remove: { width: 28, height: 28, borderRadius: 14, alignItems: "center", justifyContent: "center", backgroundColor: "#8882" },
  pressed: { opacity: 0.6 },
  removeText: { color: "#888", fontSize: 12 },
});
