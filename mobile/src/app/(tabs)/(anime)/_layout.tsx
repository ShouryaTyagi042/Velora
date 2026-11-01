import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function AnimeStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="anime" options={{ title: "Anime" }} />
    </Stack>
  );
}
