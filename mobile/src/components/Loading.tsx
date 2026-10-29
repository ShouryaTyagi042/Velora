import { ActivityIndicator, StyleSheet, View } from "react-native";

import { space } from "@/theme";

// Loading is for waits of unknown shape (the player, a retry). Content with a known shape
// shows a Skeleton instead.
export function Loading() {
  return (
    <View style={styles.center} accessibilityLabel="Loading">
      <ActivityIndicator size="large" />
    </View>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", padding: space.xl },
});
