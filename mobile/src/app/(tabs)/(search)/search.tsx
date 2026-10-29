import { router, Stack } from "expo-router";
import { useState } from "react";
import { FlatList, StyleSheet, View } from "react-native";

import { listMedia, type Media } from "@/api/media";
import { EmptyView } from "@/components/EmptyView";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { MediaCard } from "@/components/MediaCard";
import { useAsync } from "@/hooks/useAsync";
import { space } from "@/theme";

const fetchAll = (signal: AbortSignal) => listMedia({ signal });
const openMedia = (m: Media) => router.push({ pathname: "/media/[id]", params: { id: m.id } });
const renderCard = ({ item }: { item: Media }) => (
  <View style={styles.cell}>
    <MediaCard media={item} onPress={openMedia} />
  </View>
);

// matches is a case- and accent-insensitive "contains" on the title and the actors' names.
// The whole library is a few hundred items, so searching on the phone is instant and needs no
// server endpoint; a large library would move this into SQL.
function matches(m: Media, q: string): boolean {
  const norm = (s: string) => s.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();
  const needle = norm(q);
  return norm(m.title).includes(needle) || (m.actors ?? []).some((a) => norm(a.name).includes(needle));
}

// Search: the native search field (in the glass tab bar on iOS 26, in the header elsewhere).
export default function Search() {
  const { state, refetch } = useAsync(fetchAll);
  const [query, setQuery] = useState("");
  const q = query.trim();

  const header = (
    <Stack.Screen
      options={{
        headerSearchBarOptions: {
          placeholder: "Titles and actors",
          onChangeText: (e) => setQuery(e.nativeEvent.text),
          autoCapitalize: "none",
        },
      }}
    />
  );

  if (state.status === "loading") return <>{header}<Loading /></>;
  if (state.status === "error") return <>{header}<ErrorView error={state.error} onRetry={refetch} /></>;

  const results = q ? state.data.filter((m) => matches(m, q)) : [];
  return (
    <>
      {header}
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        keyboardDismissMode="on-drag"
        data={results}
        keyExtractor={(m) => m.id}
        renderItem={renderCard}
        numColumns={2}
        contentContainerStyle={styles.grid}
        ListEmptyComponent={
          q ? (
            <EmptyView title={`Nothing matches “${q}”`} detail="Search looks at titles and actors' names. Check the spelling, or try part of a name." />
          ) : (
            <EmptyView title="Search your library" detail={`Find a film, a comic or an actor among ${state.data.length} items.`} />
          )
        }
      />
    </>
  );
}

const styles = StyleSheet.create({
  grid: { paddingHorizontal: space.sm, paddingBottom: space.xl, flexGrow: 1 },
  cell: { width: "50%", padding: space.sm - 2 },
});
