import { Text as RNText, type TextProps } from "react-native";

import { typography, useAppTheme, type TypeRole } from "@/theme";

// variant, not role: TextProps already has an ARIA `role`.
type Props = TextProps & { variant?: TypeRole; muted?: boolean; color?: string };

// Text is the type scale as a component: pick a role, not a font size. Text grows with the
// system's text size (Dynamic Type), so layouts around it must let it wrap.
export function Text({ variant = "body", muted = false, color, style, ...rest }: Props) {
  const { colors } = useAppTheme();
  return <RNText {...rest} style={[typography[variant], { color: color ?? (muted ? colors.textMuted : colors.text) }, style]} />;
}
