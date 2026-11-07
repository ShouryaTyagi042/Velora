import { ActivityIndicator, StyleSheet } from "react-native";

import type { Media, Page } from "@/api/media";
import { usePages } from "@/hooks/usePages";
import { useRescan } from "@/hooks/useRescan";
import { openMedia } from "@/navigation/openMedia";
import { space } from "@/theme";
import { Button } from "./Button";
import { GroupList } from "./GroupList";
import { MediaCard } from "./MediaCard";

type Props = {
  // Must be stable unless its inputs change (useCallback): a new one starts the list again.
  fetchPage: (after: string | undefined, signal: AbortSignal) => Promise<Page<Media>>;
  empty: { title: string; detail?: string };
  header?: React.ReactElement;
};

const renderCard = (m: Media) => <MediaCard media={m} onPress={openMedia} layout="wide" />;

// VideoList is a paged, single-column list of videos: one full-width card per row, the next page
// loading as the end comes into view, pull down to rescan.
export function VideoList({ fetchPage, empty, header }: Props) {
  const { state, refreshing, refetch, loadMore, loadingMore, moreError } = usePages(fetchPage);
  const { scanning, rescan } = useRescan(refetch);
  return (
    <GroupList
      state={state}
      keyOf={(m) => m.id}
      render={renderCard}
      aspectRatio={16 / 9}
      refreshing={scanning || refreshing}
      onRefresh={rescan}
      onRetry={refetch}
      onEndReached={loadMore}
      header={header}
      empty={empty}
      footer={
        loadingMore ? (
          <ActivityIndicator style={styles.footer} accessibilityLabel="Loading more" />
        ) : moreError ? (
          // onEndReached won't fire again for content that hasn't grown, so retry is a button.
          <Button label="Couldn't load more. Try again" variant="quiet" onPress={loadMore} style={styles.footer} />
        ) : undefined
      }
    />
  );
}

const styles = StyleSheet.create({
  footer: { marginTop: space.xl },
});
