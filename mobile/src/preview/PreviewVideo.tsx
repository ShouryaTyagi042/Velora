import { useFocusEffect } from "expo-router";
import { VideoView } from "expo-video";
import { useCallback, useEffect } from "react";
import { StyleSheet } from "react-native";

import { usePreviewPlayer } from "./PreviewProvider";
import { stopPreview } from "./store";

// PreviewVideo is the shared player drawn over the previewing card's poster. It is mounted only
// while this card is the one previewing. Leaving the screen (another tab, or Details pushed on
// top by tapping the card) or the card unmounting (scrolled far away) ends the preview.
export function PreviewVideo({ id }: { id: string }) {
  const player = usePreviewPlayer();
  useFocusEffect(useCallback(() => () => stopPreview(id), [id]));
  useEffect(() => () => stopPreview(id), [id]);
  return (
    <VideoView
      player={player}
      style={StyleSheet.absoluteFill}
      contentFit="cover"
      nativeControls={false}
      allowsPictureInPicture={false}
      pointerEvents="none"
    />
  );
}
