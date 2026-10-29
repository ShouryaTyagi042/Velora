import { DarkTheme, DefaultTheme, Stack, ThemeProvider } from "expo-router";
import { useEffect } from "react";
import { StyleSheet, useColorScheme } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import { reconcile } from "@/progress/sync";
import { ReaderSettingsProvider } from "@/reader/settings";

// The Stack is the OS's navigation: push slides a screen in, back (button, swipe, Android
// back key) pops it. Each file in src/app is one screen in it.
// GestureHandlerRootView must wrap everything that uses gestures (the reader's pinch-zoom).
export default function RootLayout() {
  const scheme = useColorScheme();
  // Push any progress a previous session saved locally but never got to the server.
  useEffect(() => {
    void reconcile();
  }, []);
  return (
    <GestureHandlerRootView style={styles.root}>
      <ThemeProvider value={scheme === "dark" ? DarkTheme : DefaultTheme}>
        <ReaderSettingsProvider>
          <Stack>
            <Stack.Screen name="index" options={{ title: "Velora" }} />
            <Stack.Screen name="library" options={{ title: "Library" }} />
            <Stack.Screen name="media/[id]" options={{ title: "" }} />
            <Stack.Screen name="player/[id]" options={{ title: "" }} />
            <Stack.Screen name="reader/[id]" options={{ headerShown: false }} />
          </Stack>
        </ReaderSettingsProvider>
      </ThemeProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({ root: { flex: 1 } });
