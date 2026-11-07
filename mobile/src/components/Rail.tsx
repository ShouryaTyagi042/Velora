import { router, type Href } from "expo-router";
import { FlatList, Pressable, StyleSheet, View } from "react-native";

import { space, useAppTheme } from "@/theme";
import { Icon } from "./Icon";
import { Text } from "./Text";

type Props<T> = {
  title: string;
  items: T[];
  keyOf: (item: T) => string;
  render: (item: T) => React.ReactElement;
  seeAll?: Href; // a heading that leads somewhere shows a chevron and opens it
};

// Rail is a heading and a horizontal row of cards: the unit Home is built from. An empty rail
// isn't drawn at all.
export function Rail<T>({ title, items, keyOf, render, seeAll }: Props<T>) {
  const { colors } = useAppTheme();
  if (items.length === 0) return null;
  return (
    <View style={styles.section}>
      <Pressable
        disabled={!seeAll}
        onPress={() => seeAll && router.push(seeAll)}
        accessibilityRole={seeAll ? "button" : "header"}
        accessibilityLabel={seeAll ? `${title}, see all` : title}
        style={styles.heading}
        hitSlop={{ top: 8, bottom: 8 }}
      >
        <Text variant="title">{title}</Text>
        {seeAll && <Icon sf="chevron.right" md="chevron_right" size={14} color={colors.textMuted} />}
      </Pressable>
      <FlatList
        horizontal
        data={items}
        keyExtractor={keyOf}
        renderItem={({ item }) => render(item)}
        contentContainerStyle={styles.row}
        showsHorizontalScrollIndicator={false}
        decelerationRate="fast"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  section: { gap: space.sm + 2 },
  heading: { flexDirection: "row", alignItems: "center", gap: space.sm, marginHorizontal: space.lg, alignSelf: "flex-start" },
  row: { paddingHorizontal: space.lg, gap: space.sm },
});
