import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";

import { listMedia, MediaKind } from "@/api/media";
import { GroupHeader } from "@/components/GroupHeader";
import { VideoList } from "@/components/VideoList";

// An actor's page: everything they're in, newest first, one per row. Any section: a voice actor
// added by hand to anime episodes lands here too. Shuffle plays one of theirs at random.
export default function Actor() {
  const { name } = useLocalSearchParams<{ name: string }>();
  const fetchPage = useCallback(
    (after: string | undefined, signal: AbortSignal) => listMedia({ kind: MediaKind.Video, actor: name, sort: "recent", after, signal }),
    [name],
  );
  const shuffle = async () => {
    const [pick] = (await listMedia({ kind: MediaKind.Video, actor: name, sort: "random", limit: 1 })).items;
    if (pick) router.push({ pathname: "/player/[id]", params: { id: pick.id } });
  };

  return (
    <>
      <Stack.Screen options={{ title: name }} />
      <VideoList
        fetchPage={fetchPage}
        header={<GroupHeader action={{ label: "Shuffle play", icon: { sf: "shuffle", md: "shuffle" }, onPress: () => void shuffle() }} />}
        empty={{ title: `Nothing with ${name}`, detail: "The files may have been moved or renamed. Pull down to rescan." }}
      />
    </>
  );
}
