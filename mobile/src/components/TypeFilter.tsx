import { router } from "expo-router";
import { ScrollView, StyleSheet } from "react-native";

import { MEDIA_TYPES, type MediaType } from "@/lib/mediaType";
import { space } from "@/theme";
import { Chip } from "./Chip";

// inset is the side padding of the list it sits in, which the pills bleed through to the edge.
type Props = { value: MediaType; options?: typeof MEDIA_TYPES; inset?: number };

// TypeFilter is the row of pills at the top of a personal list: All, Videos, Anime, Comics. The
// choice lives in the screen's URL (?type=), so back restores it and nothing else has to.
export function TypeFilter({ value, options = MEDIA_TYPES, inset = space.lg }: Props) {
  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      contentContainerStyle={[styles.row, { paddingHorizontal: inset }]}
      style={{ marginHorizontal: -inset }}
    >
      {options.map((o) => (
        <Chip
          key={o.type}
          label={o.label}
          selected={value === o.type}
          accessibilityLabel={`Show ${o.type === "all" ? "everything" : o.label.toLowerCase()}`}
          onPress={() => router.setParams({ type: o.type === "all" ? undefined : o.type })}
        />
      ))}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { gap: space.sm },
});
