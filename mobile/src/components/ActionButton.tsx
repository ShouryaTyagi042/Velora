import * as Haptics from "expo-haptics";
import type { AndroidSymbol, SFSymbol } from "expo-symbols";
import { Pressable, StyleSheet } from "react-native";

import { space, touch, useAppTheme } from "@/theme";
import { Icon } from "./Icon";
import { Text } from "./Text";

type Props = {
  label: string;
  icon: { sf: SFSymbol; md: AndroidSymbol };
  onPress: () => void;
  active?: boolean; // on: drawn in Velora blue (a favourite)
  tone?: "default" | "danger";
  accessibilityLabel?: string;
  disabled?: boolean;
};

// ActionButton is an icon over a short label, in a row under Details' Play button: the quick
// things you do to an item (Favourite, Tag, Bin), the way a streaming app shows My List and Share.
// Each taps with a light haptic, since each changes something.
export function ActionButton({ label, icon, onPress, active = false, tone = "default", accessibilityLabel, disabled = false }: Props) {
  const { colors } = useAppTheme();
  const color = active ? colors.primaryText : tone === "danger" ? colors.danger : colors.text;
  return (
    <Pressable
      onPress={() => {
        void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
        onPress();
      }}
      disabled={disabled}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? label}
      accessibilityState={{ selected: active, disabled }}
      style={({ pressed }) => [styles.button, (pressed || disabled) && styles.dimmed]}
    >
      <Icon sf={icon.sf} md={icon.md} size={24} color={color} />
      <Text variant="caption" color={active ? colors.primaryText : colors.textMuted} numberOfLines={1}>
        {label}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: { flex: 1, minHeight: touch.min + 12, alignItems: "center", justifyContent: "center", gap: space.xs },
  dimmed: { opacity: 0.5 },
});
