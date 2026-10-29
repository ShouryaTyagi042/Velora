import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function HomeStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="index" options={{ title: "Velora" }} />
    </Stack>
  );
}
