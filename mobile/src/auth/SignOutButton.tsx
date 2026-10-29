import { Pressable, StyleSheet } from "react-native";

import { Icon } from "@/components/Icon";
import { touch, useAppTheme } from "@/theme";
import { useAuth } from "./AuthProvider";

// SignOutButton sits in the Home header. Signing out ends this phone's session on the server and
// forgets the token; the root layout then shows the login screen.
export function SignOutButton() {
  const { colors } = useAppTheme();
  const auth = useAuth();
  const user = auth.status === "signedIn" ? auth.user : null;
  return (
    <Pressable
      onPress={() => void auth.signOut()}
      hitSlop={8}
      accessibilityRole="button"
      accessibilityLabel={user ? `Sign out ${user.username}` : "Sign out"}
      style={({ pressed }) => [styles.button, pressed && styles.pressed]}
    >
      <Icon sf="rectangle.portrait.and.arrow.right" md="logout" size={20} color={colors.primary} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: { width: touch.min, height: touch.min, alignItems: "center", justifyContent: "center" },
  pressed: { opacity: 0.6 },
});
