import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function SearchStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="search" options={{ title: "Search" }} />
    </Stack>
  );
}
