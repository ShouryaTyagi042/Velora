import { router } from "expo-router";
import { Pressable, StyleSheet } from "react-native";

import { Text } from "@/components/Text";
import { radius, touch, useAppTheme } from "@/theme";
import { useAuth } from "./AuthProvider";

// ProfileButton sits in Home's header: the signed-in person's initial on a Velora-blue square,
// the way a streaming app shows whose profile this is. It opens You (favourites, tags, bin, sign out).
export function ProfileButton() {
  const { colors } = useAppTheme();
  const auth = useAuth();
  const name = auth.status === "signedIn" ? auth.user?.username : undefined;
  return (
    <Pressable
      onPress={() => router.push("/me")}
      hitSlop={8}
      accessibilityRole="button"
      accessibilityLabel={name ? `${name}: favourites, tags, bin and sign out` : "You: favourites, tags, bin and sign out"}
      style={({ pressed }) => [styles.button, pressed && styles.pressed]}
    >
      <Text variant="label" color={colors.onPrimary} style={[styles.avatar, { backgroundColor: colors.primary }]}>
        {(name ?? "?").slice(0, 1).toUpperCase()}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: { width: touch.min, height: touch.min, alignItems: "center", justifyContent: "center" },
  avatar: { width: 30, height: 30, lineHeight: 30, textAlign: "center", borderRadius: radius.md, overflow: "hidden", fontWeight: "800" },
  pressed: { opacity: 0.6 },
});
