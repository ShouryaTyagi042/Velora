import { isGlassEffectAPIAvailable, isLiquidGlassAvailable } from "expo-glass-effect";
import { useEffect, useState } from "react";
import { AccessibilityInfo, Platform } from "react-native";

// Liquid Glass exists only on iOS 26+. isGlassEffectAPIAvailable guards against iOS 26 betas that
// lack the API and crash. Both return false on Android and web. Checked once: neither changes
// while the app runs.
const SUPPORTED = Platform.OS === "ios" && isLiquidGlassAvailable() && isGlassEffectAPIAvailable();

// useGlass says whether to draw real glass right now. Reduce Transparency is the user asking for
// less of it, and isLiquidGlassAvailable stays true when it's on, so it is checked separately and
// followed live (the setting can change while the app is open).
export function useGlass(): boolean {
  const [reduceTransparency, setReduceTransparency] = useState(false);

  useEffect(() => {
    if (!SUPPORTED) return;
    let alive = true;
    void AccessibilityInfo.isReduceTransparencyEnabled().then((on) => {
      if (alive) setReduceTransparency(on);
    });
    const sub = AccessibilityInfo.addEventListener("reduceTransparencyChanged", setReduceTransparency);
    return () => {
      alive = false;
      sub.remove();
    };
  }, []);

  return SUPPORTED && !reduceTransparency;
}
