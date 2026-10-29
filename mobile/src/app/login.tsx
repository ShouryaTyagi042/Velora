import { useRef, useState, type ComponentRef } from "react";
import { KeyboardAvoidingView, Platform, StyleSheet, TextInput, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { ApiError, baseUrl } from "@/api/client";
import { useAuth } from "@/auth/AuthProvider";
import { Button } from "@/components/Button";
import { Text } from "@/components/Text";
import { radius, space, touch, typography, useAppTheme } from "@/theme";

// The login screen. There's no "create account": users are made on the server with
// velora-admin, and the hint says so for whoever sets the app up the first time.
export default function Login() {
  const { colors } = useAppTheme();
  const { signIn } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const passwordRef = useRef<ComponentRef<typeof TextInput>>(null);

  const submit = () => {
    if (busy || username.trim() === "" || password === "") return;
    setBusy(true);
    setError(null);
    // On success the root layout's guard swaps this screen for the library; nothing to do here.
    signIn(username.trim(), password).catch((err: unknown) => {
      setError(describe(err));
      setBusy(false);
    });
  };

  const input = [styles.input, { color: colors.text, backgroundColor: colors.surface }];
  return (
    <SafeAreaView style={[styles.screen, { backgroundColor: colors.background }]}>
      <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={styles.center}>
        <View style={styles.form}>
          <Text variant="display" accessibilityRole="header">
            Velora
          </Text>
          <Text muted>Sign in to your library.</Text>

          <TextInput
            value={username}
            onChangeText={setUsername}
            placeholder="Username"
            placeholderTextColor={colors.textMuted}
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="username"
            textContentType="username"
            returnKeyType="next"
            onSubmitEditing={() => passwordRef.current?.focus()}
            submitBehavior="submit"
            accessibilityLabel="Username"
            style={input}
          />
          <TextInput
            ref={passwordRef}
            value={password}
            onChangeText={setPassword}
            placeholder="Password"
            placeholderTextColor={colors.textMuted}
            secureTextEntry
            autoComplete="current-password"
            textContentType="password"
            returnKeyType="go"
            onSubmitEditing={submit}
            accessibilityLabel="Password"
            style={input}
          />
          {error && (
            <Text variant="caption" color={colors.danger} accessibilityLiveRegion="polite">
              {error}
            </Text>
          )}
          <Button
            label={busy ? "Signing in…" : "Sign in"}
            onPress={submit}
            disabled={busy || username.trim() === "" || password === ""}
            haptic
          />
          <Text variant="caption" muted style={styles.hint}>
            No account? Create one on the server: make user NAME=yourname
          </Text>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

// describe says what went wrong and what to do about it.
function describe(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return "That username and password don't match. Check them and try again.";
    if (err.status === 429) return "Too many attempts. Wait a minute, then try again.";
    return `${err.message}. Try again; if it keeps happening, check the server's log.`;
  }
  return `Can't reach the server at ${baseUrl}. Check that velora-server is running and this phone is on the same Wi-Fi.`;
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  center: { flex: 1, justifyContent: "center" },
  form: { padding: space.xl, gap: space.md, width: "100%", maxWidth: 420, alignSelf: "center" },
  input: { ...typography.body, borderRadius: radius.lg, paddingHorizontal: space.md, minHeight: touch.min },
  hint: { textAlign: "center", marginTop: space.sm },
});
