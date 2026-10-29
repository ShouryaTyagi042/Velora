import { DarkTheme, DefaultTheme, type Theme } from "expo-router";
import { useColorScheme } from "react-native";

import { palette, type Colors } from "./tokens";

// useAppTheme is the app's colours for the current light/dark setting.
export function useAppTheme(): { colors: Colors; dark: boolean } {
  const dark = useColorScheme() === "dark";
  return { colors: dark ? palette.dark : palette.light, dark };
}

// navigationTheme builds the router's theme (headers, tab bar) from the same tokens, so the
// header and the screens under it can never disagree.
export function navigationTheme(dark: boolean): Theme {
  const base = dark ? DarkTheme : DefaultTheme;
  const c = dark ? palette.dark : palette.light;
  return {
    ...base,
    colors: {
      ...base.colors,
      primary: c.primary,
      background: c.background,
      card: c.background,
      text: c.text,
      border: c.border,
      notification: c.danger,
    },
  };
}
