import { Stack, ThemeProvider } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { useEffect } from "react";
import { StyleSheet } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import { AuthProvider, useAuth } from "@/auth/AuthProvider";
import { ReaderSettingsProvider } from "@/reader/settings";
import { navigationTheme, useAppTheme } from "@/theme";

// Keep the splash up until we know whether there's a session, so the login screen never
// flashes on a phone that's already signed in.
void SplashScreen.preventAutoHideAsync();

// The root stack holds the tabs and, above them, the screens you go *into*: Details, the player
// and the reader cover the tab bar, as a film or a book should. Back (button, swipe, Android back
// key) returns to whichever tab you came from.
// GestureHandlerRootView must wrap everything that uses gestures (the reader's pinch-zoom).
export default function RootLayout() {
  const { dark } = useAppTheme();
  return (
    <GestureHandlerRootView style={styles.root}>
      <ThemeProvider value={navigationTheme(dark)}>
        <AuthProvider>
          <ReaderSettingsProvider>
            <RootStack />
          </ReaderSettingsProvider>
        </AuthProvider>
      </ThemeProvider>
    </GestureHandlerRootView>
  );
}

// RootStack shows the library only with a session and the login screen only without one.
// Stack.Protected redirects by itself when the session changes: signing in, signing out, or a
// 401 from an expired token all land on the right screen with no navigation code here.
function RootStack() {
  const { status } = useAuth();
  const signedIn = status === "signedIn";

  useEffect(() => {
    if (status !== "loading") SplashScreen.hide();
  }, [status]);
  if (status === "loading") return null; // the splash is still showing

  return (
    <Stack>
      <Stack.Protected guard={signedIn}>
        <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
        <Stack.Screen name="media/[id]" options={{ title: "", headerBackButtonDisplayMode: "minimal" }} />
        <Stack.Screen name="player/[id]" options={{ title: "", headerBackButtonDisplayMode: "minimal" }} />
        <Stack.Screen name="reader/[id]" options={{ headerShown: false }} />
      </Stack.Protected>
      <Stack.Protected guard={!signedIn}>
        <Stack.Screen name="login" options={{ headerShown: false }} />
      </Stack.Protected>
    </Stack>
  );
}

const styles = StyleSheet.create({ root: { flex: 1 } });
