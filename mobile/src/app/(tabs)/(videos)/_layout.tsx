import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function VideosStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="videos" options={{ title: "Videos" }} />
    </Stack>
  );
}
