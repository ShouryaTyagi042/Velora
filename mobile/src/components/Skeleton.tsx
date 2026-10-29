import { useEffect } from "react";
import { type DimensionValue, type StyleProp, type ViewStyle } from "react-native";
import Animated, {
  cancelAnimation,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";

import { radius, useAppTheme } from "@/theme";

type Props = { width?: DimensionValue; height?: DimensionValue; aspectRatio?: number; style?: StyleProp<ViewStyle> };

// Skeleton is a grey block in the shape of what will appear there: the screen shows where the
// content goes instead of a spinner, and nothing jumps when it arrives. It pulses gently, or
// stays still with Reduce Motion on. Hidden from screen readers; the screen says "Loading".
export function Skeleton({ width = "100%", height, aspectRatio, style }: Props) {
  const { colors } = useAppTheme();
  const reduceMotion = useReducedMotion();
  const opacity = useSharedValue(1);

  useEffect(() => {
    if (reduceMotion) return;
    opacity.set(withRepeat(withTiming(0.5, { duration: 800 }), -1, true));
    return () => cancelAnimation(opacity);
  }, [opacity, reduceMotion]);

  const pulse = useAnimatedStyle(() => ({ opacity: opacity.get() }));
  return (
    <Animated.View
      importantForAccessibility="no-hide-descendants"
      accessibilityElementsHidden
      style={[{ width, height, aspectRatio, backgroundColor: colors.skeleton, borderRadius: radius.md }, pulse, style]}
    />
  );
}
