import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

// Home draws its own header (the wordmark and the shortcuts over the billboard); the screens it
// leads to use the standard large-title one.
export default function HomeStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="index" options={{ headerShown: false, title: "Home" }} />
      <Stack.Screen name="comics" options={{ title: "Comics" }} />
    </Stack>
  );
}
