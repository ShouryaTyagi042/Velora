import { Stack } from "expo-router";

export default function ShortsStack() {
  return (
    <Stack>
      <Stack.Screen name="shorts" options={{ headerShown: false, title: "Shorts" }} />
    </Stack>
  );
}
