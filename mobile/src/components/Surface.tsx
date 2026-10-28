import { GlassView, type GlassColorScheme, type GlassStyle } from "expo-glass-effect";
import { View, type StyleProp, type ViewStyle } from "react-native";

import { useGlass } from "@/hooks/useGlass";

// Liquid Glass is for the controls layer only (headers, bars, buttons over content), never for
// content itself. Each role is one decision: which glass, and what to draw where there is none.
// The fallback colours move into the design tokens in phase 10b.
const ROLES = {
  bar: { style: "regular", interactive: false, fallback: "rgba(28, 28, 30, 0.82)" },
  control: { style: "clear", interactive: true, fallback: "rgba(0, 0, 0, 0.5)" },
} satisfies Record<string, { style: GlassStyle; interactive: boolean; fallback: string }>;

export type SurfaceRole = keyof typeof ROLES;

type Props = {
  role: SurfaceRole;
  // false fades the glass out natively. Glass doesn't render below opacity 1, so hiding it is a
  // style change, not an opacity animation; fade the contents separately.
  visible?: boolean;
  colorScheme?: GlassColorScheme;
  style?: StyleProp<ViewStyle>;
  children?: React.ReactNode;
};

// Surface is the only place that decides between glass and its fallback: GlassView on iOS 26+,
// a solid colour on Android, older iOS, web, and with Reduce Transparency on. Screens never use
// GlassView directly.
export function Surface({ role, visible = true, colorScheme = "auto", style, children }: Props) {
  const glass = useGlass();
  const r = ROLES[role];

  if (!glass) {
    return <View style={[{ backgroundColor: visible ? r.fallback : "transparent" }, style]}>{children}</View>;
  }
  return (
    <GlassView
      glassEffectStyle={{ style: visible ? r.style : "none", animate: true }}
      isInteractive={r.interactive}
      colorScheme={colorScheme}
      style={style}
    >
      {children}
    </GlassView>
  );
}
