import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function AnimeStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="anime" options={{ title: "Anime" }} />
      <Stack.Screen name="series/[name]" options={{ title: "" }} />
      <Stack.Screen name="episodes" options={{ title: "All anime" }} />
    </Stack>
  );
}
