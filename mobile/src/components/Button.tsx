import * as Haptics from "expo-haptics";
import type { AndroidSymbol, SFSymbol } from "expo-symbols";
import { Pressable, StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { radius, space, touch, useAppTheme } from "@/theme";
import { Icon } from "./Icon";
import { Text } from "./Text";

type Variant = "play" | "primary" | "secondary" | "quiet" | "danger";

type Props = {
  label: string;
  onPress: () => void;
  // play: the white button that starts something (Play, Resume, Read). primary: Velora blue, for
  // the one action of a form. secondary: translucent grey, beside play. quiet: outline only.
  // danger: red, for what can't be undone (Delete forever, Empty bin).
  variant?: Variant;
  icon?: { sf: SFSymbol; md: AndroidSymbol };
  disabled?: boolean;
  haptic?: boolean; // a light tap on meaningful actions (Play, Read), not on every button
  accessibilityHint?: string;
  style?: StyleProp<ViewStyle>;
};

// Button: 44 pt tall at least, pressed feedback, one look per variant.
export function Button({ label, onPress, variant = "primary", icon, disabled = false, haptic = false, accessibilityHint, style }: Props) {
  const { colors } = useAppTheme();
  const look = {
    play: { bg: colors.play, fg: colors.onPlay, border: undefined },
    primary: { bg: colors.primary, fg: colors.onPrimary, border: undefined },
    secondary: { bg: colors.secondaryFill, fg: colors.text, border: undefined },
    quiet: { bg: "transparent", fg: colors.text, border: colors.border },
    danger: { bg: colors.dangerFill, fg: colors.onPrimary, border: undefined },
  }[variant];
  const press = () => {
    if (haptic) void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
    onPress();
  };
  return (
    <Pressable
      onPress={press}
      disabled={disabled}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled }}
      accessibilityHint={accessibilityHint}
      style={({ pressed }) => [
        styles.button,
        { backgroundColor: look.bg },
        look.border && { borderColor: look.border, borderWidth: StyleSheet.hairlineWidth * 2 },
        pressed && styles.pressed,
        disabled && styles.disabled,
        style,
      ]}
    >
      <View style={styles.row}>
        {icon && <Icon sf={icon.sf} md={icon.md} size={18} color={look.fg} />}
        <Text variant="body" color={look.fg} style={styles.label} numberOfLines={1}>
          {label}
        </Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    minHeight: touch.min,
    paddingHorizontal: space.lg,
    borderRadius: radius.md,
    alignItems: "center",
    justifyContent: "center",
  },
  row: { flexDirection: "row", alignItems: "center", gap: space.sm },
  label: { fontWeight: "700" },
  pressed: { opacity: 0.75, transform: [{ scale: 0.98 }] },
  disabled: { opacity: 0.45 },
});
