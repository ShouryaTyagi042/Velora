import { LinearGradient } from "expo-linear-gradient";
import { StyleSheet, type StyleProp, type ViewStyle } from "react-native";

type Props = { colors: readonly [string, string, ...string[]]; style?: StyleProp<ViewStyle> };

// Gradient is a vertical fade (top to bottom) laid over artwork. It never takes touches.
export function Gradient({ colors, style }: Props) {
  return <LinearGradient colors={colors} style={[StyleSheet.absoluteFill, style]} pointerEvents="none" />;
}
