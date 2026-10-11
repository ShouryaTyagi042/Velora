import { useTheme } from "expo-router";
import { StyleSheet, Text, View } from "react-native";

type Props = { title: string; detail?: string };

// EmptyView is the fourth state: the request worked and there is nothing to show.
export function EmptyView({ title, detail }: Props) {
  const { colors } = useTheme();
  return (
    <View style={styles.center}>
      <Text style={[styles.title, { color: colors.text }]}>{title}</Text>
      {detail && <Text style={styles.detail}>{detail}</Text>}
    </View>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", padding: 24, gap: 8 },
  title: { fontSize: 17, fontWeight: "600", textAlign: "center" },
  detail: { color: "#888", textAlign: "center" },
});
