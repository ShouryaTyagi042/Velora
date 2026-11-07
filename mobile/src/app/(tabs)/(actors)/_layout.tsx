import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function ActorsStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="actors" options={{ title: "Actors" }} />
      <Stack.Screen name="actor/[name]" options={{ title: "" }} />
      <Stack.Screen name="videos" options={{ title: "All videos" }} />
    </Stack>
  );
}
