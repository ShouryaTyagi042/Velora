import { LinearGradient } from "expo-linear-gradient";
import { Pressable, StyleSheet } from "react-native";

import { gradients, radius, space, useAppTheme } from "@/theme";
import { Text } from "./Text";

type Props = {
  label: string;
  selected?: boolean;
  // featured: the one pill that should stand out from its row (Surprise me), filled with the
  // brand's gradient instead of an outline.
  featured?: boolean;
  onPress?: () => void;
  accessibilityLabel?: string;
};

// Chip is a pill: a filter, a tag, a shortcut at the top of Home. Outlined at rest, filled with
// Velora blue when selected. 34 pt tall to look like a chip; hitSlop makes the target 44.
export function Chip({ label, selected = false, featured = false, onPress, accessibilityLabel }: Props) {
  const { colors } = useAppTheme();
  if (featured) {
    return (
      <Pressable
        onPress={onPress}
        disabled={!onPress}
        hitSlop={{ top: 5, bottom: 5 }}
        accessibilityRole="button"
        accessibilityLabel={accessibilityLabel}
        style={({ pressed }) => pressed && styles.pressed}
      >
        <LinearGradient colors={gradients.surprise} start={{ x: 0, y: 0 }} end={{ x: 1, y: 1 }} style={[styles.chip, styles.featured]}>
          <Text variant="label" color={colors.onPrimary} style={styles.featuredLabel}>
            {label}
          </Text>
        </LinearGradient>
      </Pressable>
    );
  }
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      hitSlop={{ top: 5, bottom: 5 }}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      accessibilityState={{ selected }}
      style={({ pressed }) => [
        styles.chip,
        { borderColor: selected ? colors.primary : colors.chipBorder },
        selected && { backgroundColor: colors.primary },
        pressed && { backgroundColor: selected ? colors.primary : colors.surfaceRaised },
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
    minHeight: 34,
    justifyContent: "center",
    borderWidth: 1,
    borderRadius: radius.pill,
    paddingHorizontal: space.md + 2,
  },
  featured: { borderColor: "transparent", overflow: "hidden" },
  featuredLabel: { fontWeight: "800" },
  pressed: { opacity: 0.75 },
});
