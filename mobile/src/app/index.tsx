import { useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { getJSON } from "@/api/client";

type Health =
  | { state: "loading" }
  | { state: "ok"; status: string }
  | { state: "error"; message: string };

export default function Index() {
  const [health, setHealth] = useState<Health>({ state: "loading" });

  useEffect(() => {
    getJSON<{ status: string }>("/health")
      .then((body) => setHealth({ state: "ok", status: body.status }))
      .catch((err: Error) => setHealth({ state: "error", message: err.message }));
  }, []);

  return (
    <View style={styles.container}>
      {health.state === "loading" && <Text>Checking server…</Text>}
      {health.state === "ok" && <Text>Server says: {health.status}</Text>}
      {health.state === "error" && <Text>Server unreachable: {health.message}</Text>}
      <Text style={styles.url}>{process.env.EXPO_PUBLIC_API_URL}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    color: "#00000088",
    gap: 8,
  },
  url: {
    color: "#888",
    fontSize: 16,
  },
});
