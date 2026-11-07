import type { AndroidSymbol, SFSymbol } from "expo-symbols";
import { StyleSheet, View } from "react-native";

import { space } from "@/theme";
import { Button } from "./Button";
import { Text } from "./Text";

type Props = {
  lede?: string;
  action: { label: string; icon: { sf: SFSymbol; md: AndroidSymbol }; onPress: () => void };
};

// GroupHeader sits at the top of a group's list (an actor, a series): a line about it and the one
// thing to do with all of it, as a white Play-style button.
export function GroupHeader({ lede, action }: Props) {
  return (
    <View style={styles.header}>
      {lede && <Text muted>{lede}</Text>}
      <Button label={action.label} icon={action.icon} variant="play" haptic onPress={action.onPress} />
    </View>
  );
}

const styles = StyleSheet.create({
  header: { gap: space.md, marginBottom: space.xl },
});
