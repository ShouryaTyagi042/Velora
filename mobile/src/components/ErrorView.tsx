import { StyleSheet, View } from "react-native";

import { ApiError, baseUrl } from "@/api/client";
import { PlaybackError } from "@/hooks/usePlayerStatus";
import { space } from "@/theme";
import { Button } from "./Button";
import { Text } from "./Text";

type Props = { error: Error; onRetry?: () => void };

// ErrorView turns any failure into what happened, why, and what to do: words a person can act
// on, plus a retry button.
export function ErrorView({ error, onRetry }: Props) {
  const { title, detail } = describe(error);
  return (
    <View style={styles.center}>
      <Text variant="title" style={styles.text}>
        {title}
      </Text>
      <Text muted style={styles.text}>
        {detail}
      </Text>
      {onRetry && <Button label="Try again" onPress={onRetry} style={styles.button} />}
    </View>
  );
}

function describe(error: Error): { title: string; detail: string } {
  if (error instanceof ApiError) {
    if (error.status === 404) return { title: "Not found", detail: `${error.message}. It may have been moved or renamed; go back and pull to refresh.` };
    return { title: "Something went wrong on the server", detail: `${error.message} (${error.code}). Try again; if it keeps happening, check the server's log.` };
  }
  if (error instanceof PlaybackError) return { title: "Can't play this video", detail: error.message };
  // fetch itself failed: no response at all.
  return {
    title: "Can't reach the server",
    detail: `Check that velora-server is running on your computer and that this phone is on the same Wi-Fi. The app is looking for it at ${baseUrl}.`,
  };
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", padding: space.xl, gap: space.sm },
  text: { textAlign: "center" },
  button: { marginTop: space.md },
});
