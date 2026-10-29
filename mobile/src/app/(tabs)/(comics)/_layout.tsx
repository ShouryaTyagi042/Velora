import { Stack } from "expo-router";

import { tabStackOptions } from "@/navigation/tabStack";

export default function ComicsStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="comics" options={{ title: "Comics" }} />
    </Stack>
  );
}
