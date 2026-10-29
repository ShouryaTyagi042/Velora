import type { TextStyle } from "react-native";

// Design tokens: every visual decision in the app, named for its role, never for its value.
// Components use these names only. Changing the accent is one line here; dark mode is the second
// set of colours for the same names.

const light = {
  background: "#ffffff",
  surface: "#f2f2f7", // grouped content: inputs, placeholders
  text: "#000000",
  textMuted: "#5f5f66", // 6.3:1 on background, 5.6:1 on surface
  border: "#d1d1d6",
  primary: "#0a64d8", // 5.6:1 behind white text
  onPrimary: "#ffffff",
  danger: "#c4271b",
  posterVideo: "#2b3a55", // a poster before its thumbnail arrives
  posterComic: "#553a2b",
  skeleton: "#e5e5ea",
};

const dark: typeof light = {
  background: "#000000",
  surface: "#1c1c1e",
  text: "#ffffff",
  textMuted: "#a1a1aa", // 8.1:1 on background, 6.9:1 on surface
  border: "#38383a",
  primary: "#3d8bfd",
  onPrimary: "#000000", // black reads better than white on the lighter dark-mode blue
  danger: "#ff6b5e",
  posterVideo: "#2b3a55",
  posterComic: "#553a2b",
  skeleton: "#2c2c2e",
};

export const palette = { light, dark };
export type Colors = typeof light;

// onMedia is for controls drawn over video, comic pages and posters, in either theme: that
// content is dark more often than not, and the glass under it decides the contrast.
export const onMedia = {
  text: "#ffffff",
  muted: "rgba(255, 255, 255, 0.7)",
  track: "rgba(255, 255, 255, 0.35)",
  scrim: "rgba(0, 0, 0, 0.45)", // a badge's background over an image
  backdrop: "#000000", // behind video and comic pages: letterboxing is black
  placeholderIcon: "rgba(255, 255, 255, 0.6)", // the icon on a poster with no image yet
};

// A 4-point grid. Space between groups is larger than space within them: that's how a screen
// shows what belongs together without lines or boxes.
export const space = { xs: 4, sm: 8, md: 12, lg: 16, xl: 24, xxl: 32 } as const;

export const radius = { sm: 4, md: 8, lg: 12, pill: 999 } as const;

// Five type roles. Hierarchy comes from the differences between them, so there are few, and
// they're far apart.
export const typography = {
  display: { fontSize: 28, lineHeight: 34, fontWeight: "700" },
  title: { fontSize: 20, lineHeight: 25, fontWeight: "700" },
  body: { fontSize: 16, lineHeight: 22, fontWeight: "400" },
  label: { fontSize: 14, lineHeight: 19, fontWeight: "500" },
  caption: { fontSize: 12, lineHeight: 16, fontWeight: "400" },
} satisfies Record<string, TextStyle>;
export type TypeRole = keyof typeof typography;

// Glass roles (Surface): which glass, and the solid colour drawn where there is none (Android,
// iOS before 26, web, Reduce Transparency).
export const glass = {
  bar: { style: "regular", interactive: false, fallback: "rgba(28, 28, 30, 0.82)" },
  control: { style: "clear", interactive: true, fallback: "rgba(0, 0, 0, 0.5)" },
} as const;

// The minimum size of anything you tap: 44 pt (Apple) and 48 dp (Android) are met with hitSlop
// where the visual is smaller.
export const touch = { min: 44 } as const;

export const motion = { fast: 150, normal: 250 } as const;
