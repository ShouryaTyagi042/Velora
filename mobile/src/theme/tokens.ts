import type { TextStyle } from "react-native";

// Design tokens: every visual decision in the app, named for its role, never for its value.
// Components use these names only.
//
// Velora is a dark, cinema-first app: one theme, tuned for an iPhone in a dark room. The base is
// an ink with a little blue in it rather than flat black, so the Velora blue reads as light coming
// out of the screen, not a sticker on top of it. Contrast ratios are against `background`.

const colors = {
  background: "#05070c",
  surface: "#0f131c", // inputs, sheets, grouped rows
  surfaceRaised: "#181e2b", // a pressed row, a chip, the secondary button
  text: "#f5f7fb", // 18.6:1
  textMuted: "#9ba4b6", // 7.9:1; 6.9:1 on surface
  border: "#262d3d",
  primary: "#1f6bff", // Velora blue: fills (progress, selected chips, the brand). 4.6:1 behind white text
  primaryText: "#6ea4ff", // the blue as text or an icon on the dark base: 7.6:1
  onPrimary: "#ffffff",
  danger: "#ff5a5f", // as text or an icon: 6.3:1
  dangerFill: "#c9343a", // a destructive button's fill: 4.9:1 behind white text
  posterVideo: "#111a2e", // a poster before its thumbnail arrives
  posterComic: "#1d1628",
  skeleton: "#141a26",
  // The billboard play button: the brightest thing on the screen, as in a cinema app it should be.
  play: "#ffffff",
  onPlay: "#05070c",
  secondaryFill: "rgba(120, 132, 158, 0.32)", // "Details": quiet, translucent over artwork
  chipBorder: "rgba(245, 247, 251, 0.32)", // an unselected pill's outline, over artwork or the page
  binTint: "rgba(201, 52, 58, 0.14)", // the "in the bin" banner's wash
};

export type Colors = typeof colors;
export const palette = { dark: colors };

// onMedia is for controls and text drawn over video, comic pages and posters.
export const onMedia = {
  text: "#ffffff",
  muted: "rgba(255, 255, 255, 0.72)",
  track: "rgba(255, 255, 255, 0.3)",
  scrim: "rgba(5, 7, 12, 0.55)", // a badge's background over an image; dims a blurred backdrop
  backdrop: "#000000", // behind video and comic pages: letterboxing is black
  placeholderIcon: "rgba(255, 255, 255, 0.5)",
};

// Gradients over artwork (expo-linear-gradient colour stops). They fade into `background`, and the
// billboard's carries a trace of the blue, which is the one place the brand colour glows.
export const gradients = {
  // The bottom of a poster, under a title: transparent to near-opaque ink.
  posterBottom: ["rgba(5, 7, 12, 0)", "rgba(5, 7, 12, 0.92)"],
  // The top of the screen, under the status bar and header.
  top: ["rgba(5, 7, 12, 0.85)", "rgba(5, 7, 12, 0)"],
  // The billboard melts into the page through a blue dusk.
  billboard: ["rgba(5, 7, 12, 0)", "rgba(10, 24, 64, 0.45)", "rgba(5, 7, 12, 1)"],
  // Surprise me: the one surface filled with the brand, corner to corner.
  surprise: ["#1f6bff", "rgba(31, 107, 255, 0.25)", "#0f131c"],
} as const;

// A 4-point grid. Space between groups is larger than space within them.
export const space = { xs: 4, sm: 8, md: 12, lg: 16, xl: 24, xxl: 32, xxxl: 48 } as const;

// Artwork is nearly square-cornered, as on a film strip; controls are rounder; pills are pills.
export const radius = { sm: 4, md: 6, lg: 10, xl: 16, pill: 999 } as const;

// Type roles. SF Pro throughout (it's what iOS readers are fastest in); the personality comes from
// weight and tracking: titles are heavy and tight, labels light and open.
export const typography = {
  hero: { fontSize: 34, lineHeight: 38, fontWeight: "900", letterSpacing: -0.8 },
  display: { fontSize: 28, lineHeight: 32, fontWeight: "800", letterSpacing: -0.5 },
  title: { fontSize: 19, lineHeight: 24, fontWeight: "700", letterSpacing: -0.2 },
  body: { fontSize: 16, lineHeight: 22, fontWeight: "400" },
  label: { fontSize: 14, lineHeight: 18, fontWeight: "600" },
  caption: { fontSize: 12, lineHeight: 16, fontWeight: "500" },
} satisfies Record<string, TextStyle>;
export type TypeRole = keyof typeof typography;

// The wordmark: the brand's only typographic flourish.
export const wordmark: TextStyle = { fontSize: 24, lineHeight: 28, fontWeight: "900", letterSpacing: 3 };

// Glass roles (Surface): which glass, and the solid colour drawn where there is none.
export const glass = {
  bar: { style: "regular", interactive: false, fallback: "rgba(15, 19, 28, 0.86)" },
  control: { style: "clear", interactive: true, fallback: "rgba(0, 0, 0, 0.5)" },
} as const;

// The minimum size of anything you tap.
export const touch = { min: 44 } as const;

export const motion = { fast: 150, normal: 250, slow: 450 } as const;
