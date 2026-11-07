import { DarkTheme, type Theme } from "expo-router";

import { palette, type Colors } from "./tokens";

// useAppTheme is the app's colours. Velora has one theme, dark (app.json sets
// userInterfaceStyle "dark" so system UI agrees); this stays a hook so a second theme could
// return without touching any screen.
export function useAppTheme(): { colors: Colors; dark: true } {
  return { colors: palette.dark, dark: true };
}

// navigationTheme builds the router's theme (headers, tab bar) from the same tokens.
export function navigationTheme(): Theme {
  const c = palette.dark;
  return {
    ...DarkTheme,
    colors: {
      ...DarkTheme.colors,
      primary: c.primaryText,
      background: c.background,
      card: c.background,
      text: c.text,
      border: c.border,
      notification: c.danger,
    },
  };
}
