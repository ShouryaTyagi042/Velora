import * as Haptics from "expo-haptics";
import { Pressable, StyleSheet, type StyleProp, type ViewStyle } from "react-native";

import { radius, space, touch, useAppTheme } from "@/theme";
import { Text } from "./Text";

type Props = {
  label: string;
  onPress: () => void;
  variant?: "primary" | "secondary";
  disabled?: boolean;
  haptic?: boolean; // a light tap on meaningful actions (Play, Read), not on every button
  accessibilityHint?: string;
  style?: StyleProp<ViewStyle>;
};

// Button: 44 pt tall at least, pressed feedback, one look per variant.
export function Button({ label, onPress, variant = "primary", disabled = false, haptic = false, accessibilityHint, style }: Props) {
  const { colors } = useAppTheme();
  const primary = variant === "primary";
  const press = () => {
    if (haptic) void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
    onPress();
  };
  return (
    <Pressable
      onPress={press}
      disabled={disabled}
      accessibilityRole="button"
      accessibilityState={{ disabled }}
      accessibilityHint={accessibilityHint}
      style={({ pressed }) => [
        styles.button,
        primary ? { backgroundColor: colors.primary } : { borderColor: colors.border, borderWidth: StyleSheet.hairlineWidth },
        (pressed || disabled) && styles.dimmed,
        style,
      ]}
    >
      <Text variant="label" color={primary ? colors.onPrimary : colors.text} style={styles.label}>
        {label}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    minHeight: touch.min,
    paddingHorizontal: space.lg,
    borderRadius: radius.lg,
    alignItems: "center",
    justifyContent: "center",
  },
  label: { fontWeight: "600" },
  dimmed: { opacity: 0.6 },
});
