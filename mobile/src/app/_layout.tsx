import { DarkTheme, DefaultTheme, Stack, ThemeProvider } from "expo-router";
import { useColorScheme } from "react-native";

// The Stack is the OS's navigation: push slides a screen in, back (button, swipe, Android
// back key) pops it. Each file in src/app is one screen in it.
export default function RootLayout() {
  const scheme = useColorScheme();
  return (
    <ThemeProvider value={scheme === "dark" ? DarkTheme : DefaultTheme}>
      <Stack>
        <Stack.Screen name="index" options={{ title: "Velora" }} />
        <Stack.Screen name="library" options={{ title: "Library" }} />
        <Stack.Screen name="media/[id]" options={{ title: "" }} />
        <Stack.Screen name="player/[id]" options={{ title: "" }} />
      </Stack>
    </ThemeProvider>
  );
}
