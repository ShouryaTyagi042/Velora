import { StyleSheet, View } from "react-native";

import { space } from "@/theme";
import { Button } from "./Button";
import { Text } from "./Text";

type Props = { title: string; detail?: string; action?: { label: string; onPress: () => void } };

// EmptyView is the fourth state: the request worked and there is nothing to show. It says what
// happened, why, and (with action) what to do next.
export function EmptyView({ title, detail, action }: Props) {
  return (
    <View style={styles.center}>
      <Text variant="title" style={styles.text}>
        {title}
      </Text>
      {detail && (
        <Text muted style={styles.text}>
          {detail}
        </Text>
      )}
      {action && <Button label={action.label} onPress={action.onPress} variant="secondary" style={styles.action} />}
    </View>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", padding: space.xl, gap: space.sm },
  text: { textAlign: "center" },
  action: { marginTop: space.md },
});
