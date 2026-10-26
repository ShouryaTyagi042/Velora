import { useTheme } from "expo-router";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, baseUrl } from "@/api/client";
import { PlaybackError } from "@/hooks/usePlayerStatus";

type Props = { error: Error; onRetry?: () => void };

// ErrorView turns any failure into words a person can act on, plus a retry button.
export function ErrorView({ error, onRetry }: Props) {
  const { colors } = useTheme();
  const { title, detail } = describe(error);
  return (
    <View style={styles.center}>
      <Text style={[styles.title, { color: colors.text }]}>{title}</Text>
      <Text style={styles.detail}>{detail}</Text>
      {onRetry && (
        <Pressable style={[styles.button, { backgroundColor: colors.primary }]} onPress={onRetry}>
          <Text style={styles.buttonText}>Try again</Text>
        </Pressable>
      )}
    </View>
  );
}

function describe(error: Error): { title: string; detail: string } {
  if (error instanceof ApiError) {
    if (error.status === 404) return { title: "Not found", detail: error.message };
    return { title: "The server had a problem", detail: `${error.message} (${error.code})` };
  }
  if (error instanceof PlaybackError) return { title: "Can't play this video", detail: error.message };
  // fetch itself failed: no response at all.
  return {
    title: "Can't reach the server",
    detail: `Is velora-server running, and is this phone on the same Wi-Fi?\n${baseUrl}`,
  };
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", padding: 24, gap: 8 },
  title: { fontSize: 18, fontWeight: "600", textAlign: "center" },
  detail: { color: "#888", textAlign: "center" },
  button: { marginTop: 12, paddingHorizontal: 20, paddingVertical: 10, borderRadius: 8 },
  buttonText: { color: "#fff", fontWeight: "600" },
});
