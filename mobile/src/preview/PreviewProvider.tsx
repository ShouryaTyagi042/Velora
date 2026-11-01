import { useEventListener } from "expo";
import { useVideoPlayer, type VideoPlayer } from "expo-video";
import { createContext, useContext, useEffect } from "react";

import { authHeaders } from "@/api/client";
import { previewUrl } from "@/api/media";
import { stopPreview, useCurrentPreview } from "./store";

const PlayerContext = createContext<VideoPlayer | null>(null);

// PreviewProvider owns the one preview player, created once around all the tabs: making a native
// player is the slow part of a long-press (ADR 011), so it's paid at launch, not per press, and
// every card reuses it by swapping the source.
//
// The preview plays once, muted, and stops at its end. It also stops when another card takes it
// over, or when the previewing card loses focus or unmounts (see PreviewVideo).
export function PreviewProvider({ children }: { children: React.ReactNode }) {
  const player = useVideoPlayer(null, (p) => {
    p.muted = true;
    p.loop = false;
    p.audioMixingMode = "mixWithOthers"; // silent anyway: never interrupt music playing elsewhere
  });
  const current = useCurrentPreview();

  useEffect(() => {
    if (!current) {
      player.pause();
      return;
    }
    let live = true; // a newer press (or a stop) while this one loads wins
    void player
      .replaceAsync({ uri: previewUrl(current.media), headers: authHeaders() })
      .then(() => {
        if (live) player.play();
      })
      .catch(() => stopPreview(current.media.id)); // no preview after all: just end it
    return () => {
      live = false;
    };
  }, [current, player]);

  useEventListener(player, "playToEnd", () => stopPreview());

  return <PlayerContext.Provider value={player}>{children}</PlayerContext.Provider>;
}

// usePreviewPlayer is the shared player, for the previewing card's VideoView.
export function usePreviewPlayer(): VideoPlayer {
  const player = useContext(PlayerContext);
  if (!player) throw new Error("usePreviewPlayer outside PreviewProvider");
  return player;
}
