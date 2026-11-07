import { Stack, ThemeProvider } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { useEffect } from "react";
import { Platform, StyleSheet } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import { AuthProvider, useAuth } from "@/auth/AuthProvider";
import { PreviewProvider } from "@/preview/PreviewProvider";
import { ReaderSettingsProvider } from "@/reader/settings";
import { navigationTheme, palette } from "@/theme";

// Keep the splash up until we know whether there's a session, so the login screen never
// flashes on a phone that's already signed in.
void SplashScreen.preventAutoHideAsync();

// The root stack holds the tabs and, above them, the screens you go *into*: Details, the player
// and the reader cover the tab bar, as a film or a book should; Surprise me is a sheet over
// whatever you were doing. Back (button, swipe, Android back key) returns to where you came from.
// GestureHandlerRootView must wrap everything that uses gestures (the reader's pinch-zoom).
export default function RootLayout() {
  return (
    <GestureHandlerRootView style={styles.root}>
      <ThemeProvider value={navigationTheme()}>
        <StatusBar style="light" />
        <AuthProvider>
          <ReaderSettingsProvider>
            {/* The one long-press preview player, around every screen: cards appear in the tabs,
                and also over them (Details' rails and episodes, tag pages). */}
            <PreviewProvider>
              <RootStack />
            </PreviewProvider>
          </ReaderSettingsProvider>
        </AuthProvider>
      </ThemeProvider>
    </GestureHandlerRootView>
  );
}

// RootStack shows the library only with a session and the login screen only without one.
// Stack.Protected redirects by itself when the session changes.
function RootStack() {
  const { status } = useAuth();
  const signedIn = status === "signedIn";

  useEffect(() => {
    if (status !== "loading") SplashScreen.hide();
  }, [status]);
  if (status === "loading") return null; // the splash is still showing

  return (
    <Stack screenOptions={{ contentStyle: { backgroundColor: palette.dark.background } }}>
      <Stack.Protected guard={signedIn}>
        <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
        {/* Details draws its still under a transparent header, edge to edge. */}
        <Stack.Screen
          name="media/[id]"
          options={{ title: "", headerTransparent: true, headerBackButtonDisplayMode: "minimal", headerTintColor: palette.dark.text }}
        />
        <Stack.Screen name="player/[id]" options={{ title: "", headerBackButtonDisplayMode: "minimal" }} />
        <Stack.Screen name="reader/[id]" options={{ headerShown: false }} />
        <Stack.Screen name="tag/[name]" options={{ headerLargeTitle: true, headerTransparent: Platform.OS === "ios", headerBackButtonDisplayMode: "minimal" }} />
        <Stack.Screen
          name="surprise"
          options={{
            title: "Surprise me",
            presentation: "formSheet",
            sheetAllowedDetents: [0.86, 1],
            sheetGrabberVisible: true,
            sheetCornerRadius: 24,
            headerShown: false,
            contentStyle: { backgroundColor: palette.dark.background },
          }}
        />
      </Stack.Protected>
      <Stack.Protected guard={!signedIn}>
        <Stack.Screen name="login" options={{ headerShown: false }} />
      </Stack.Protected>
    </Stack>
  );
}

const styles = StyleSheet.create({ root: { flex: 1 } });
