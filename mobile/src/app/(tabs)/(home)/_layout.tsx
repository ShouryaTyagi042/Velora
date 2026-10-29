import { Stack } from "expo-router";

import { SignOutButton } from "@/auth/SignOutButton";
import { tabStackOptions } from "@/navigation/tabStack";

export default function HomeStack() {
  return (
    <Stack screenOptions={tabStackOptions}>
      <Stack.Screen name="index" options={{ title: "Velora", headerRight: () => <SignOutButton /> }} />
    </Stack>
  );
}
