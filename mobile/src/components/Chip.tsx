import { Pressable, StyleSheet } from "react-native";

import { radius, space, useAppTheme } from "@/theme";
import { Text } from "./Text";

type Props = { label: string; selected?: boolean; onPress?: () => void; accessibilityLabel?: string };

// Chip is a small pill button: kind filters, actor tags, the "clear filter" chip. 32 pt tall to
// look like a chip; hitSlop makes the tappable area the full 44.
export function Chip({ label, selected = false, onPress, accessibilityLabel }: Props) {
  const { colors } = useAppTheme();
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      hitSlop={{ top: 6, bottom: 6 }}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      accessibilityState={{ selected }}
      style={({ pressed }) => [
        styles.chip,
        { borderColor: selected ? colors.primary : colors.border },
        selected && { backgroundColor: colors.primary },
        pressed && styles.pressed,
      ]}
    >
      <Text variant="label" color={selected ? colors.onPrimary : colors.text}>
        {label}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  chip: {
    minHeight: 32,
    justifyContent: "center",
    borderWidth: StyleSheet.hairlineWidth * 2,
    borderRadius: radius.pill,
    paddingHorizontal: space.md,
  },
  pressed: { opacity: 0.6 },
});
