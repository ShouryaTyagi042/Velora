import { Pressable, StyleSheet } from "react-native";

import { Icon } from "@/components/Icon";
import { Text } from "@/components/Text";
import { space, touch, useAppTheme } from "@/theme";

type Props = { descending: boolean; onToggle: () => void };

// SortButton sits in a header and flips a list between A to Z and Z to A. It shows the order the
// list is in now; tapping reverses it.
export function SortButton({ descending, onToggle }: Props) {
  const { colors } = useAppTheme();
  return (
    <Pressable
      onPress={onToggle}
      hitSlop={8}
      accessibilityRole="button"
      accessibilityLabel={descending ? "Sorted Z to A" : "Sorted A to Z"}
      accessibilityHint={descending ? "Sorts A to Z" : "Sorts Z to A"}
      style={({ pressed }) => [styles.button, pressed && styles.pressed]}
    >
      <Icon sf="arrow.up.arrow.down" md="swap_vert" size={18} color={colors.primary} />
      <Text variant="label" color={colors.primary}>
        {descending ? "Z–A" : "A–Z"}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: { minHeight: touch.min, flexDirection: "row", alignItems: "center", gap: space.xs, paddingHorizontal: space.xs },
  pressed: { opacity: 0.6 },
});
