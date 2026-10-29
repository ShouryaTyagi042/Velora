import { Stack, ThemeProvider } from "expo-router";
import { useEffect } from "react";
import { StyleSheet } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import { reconcile } from "@/progress/sync";
import { ReaderSettingsProvider } from "@/reader/settings";
import { navigationTheme, useAppTheme } from "@/theme";

// The root stack holds the tabs and, above them, the screens you go *into*: Details, the player
// and the reader cover the tab bar, as a film or a book should. Back (button, swipe, Android back
// key) returns to whichever tab you came from.
// GestureHandlerRootView must wrap everything that uses gestures (the reader's pinch-zoom).
export default function RootLayout() {
  const { dark } = useAppTheme();
  // Push any progress a previous session saved locally but never got to the server.
  useEffect(() => {
    void reconcile();
  }, []);
  return (
    <GestureHandlerRootView style={styles.root}>
      <ThemeProvider value={navigationTheme(dark)}>
        <ReaderSettingsProvider>
          <Stack>
            <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
            <Stack.Screen name="media/[id]" options={{ title: "", headerBackButtonDisplayMode: "minimal" }} />
            <Stack.Screen name="player/[id]" options={{ title: "", headerBackButtonDisplayMode: "minimal" }} />
            <Stack.Screen name="reader/[id]" options={{ headerShown: false }} />
          </Stack>
        </ReaderSettingsProvider>
      </ThemeProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({ root: { flex: 1 } });
