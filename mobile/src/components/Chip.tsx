import { useTheme } from "expo-router";
import { Pressable, StyleSheet, Text } from "react-native";

type Props = { label: string; selected?: boolean; onPress?: () => void };

// Chip is a small pill button: kind filters, actor tags, the "clear filter" chip.
export function Chip({ label, selected = false, onPress }: Props) {
  const { colors } = useTheme();
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={({ pressed }) => [
        styles.chip,
        { borderColor: selected ? colors.primary : colors.border },
        selected && { backgroundColor: colors.primary },
        pressed && styles.pressed,
      ]}
    >
      <Text style={[styles.label, { color: selected ? "#fff" : colors.text }]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  chip: { borderWidth: 1, borderRadius: 999, paddingHorizontal: 12, paddingVertical: 6 },
  label: { fontSize: 14 },
  pressed: { opacity: 0.6 },
});
