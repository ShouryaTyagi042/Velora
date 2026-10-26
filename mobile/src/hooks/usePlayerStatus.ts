import { useEvent, useEventListener } from "expo";
import type { VideoPlayer } from "expo-video";
import { useState } from "react";

// PlaybackError is the native player failing (unreachable stream, a file it can't decode),
// as opposed to an ApiError from our JSON endpoints. ErrorView words the two differently.
export class PlaybackError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PlaybackError";
  }
}

export type PlayerStatus = {
  // What the player can do: "loading" covers both the first load and a mid-play rebuffer.
  status: "idle" | "loading" | "readyToPlay" | "error";
  error?: PlaybackError;
  playing: boolean; // what it is doing (or was asked to do), separate from whether it can
  buffering: boolean; // asked to play, but waiting for bytes
  ended: boolean;
  currentTime: number; // seconds
  bufferedPosition: number; // seconds; how far ahead the player has read
  duration: number; // seconds; 0 until the source has loaded
};

// usePlayerStatus turns the player's events into render state. Subscribing beats polling
// currentTime: the player pushes timeUpdate every timeUpdateEventInterval seconds, and each
// subscription is removed when the component unmounts.
//
// "Playing" is three things, and this keeps them apart: playing (the intent and the engine
// running), status (whether there are frames to show), ended. Phase 10 should save progress
// only when playing && !buffering, or it saves positions the user never watched.
export function usePlayerStatus(player: VideoPlayer): PlayerStatus {
  const { status, error } = useEvent(player, "statusChange", { status: player.status });
  const { isPlaying } = useEvent(player, "playingChange", { isPlaying: player.playing });
  const { currentTime, bufferedPosition } = useEvent(player, "timeUpdate", {
    currentTime: player.currentTime,
    bufferedPosition: player.bufferedPosition,
    currentLiveTimestamp: null,
    currentOffsetFromLive: null,
  });
  const { duration } = useEvent(player, "sourceLoad", {
    videoSource: null,
    duration: player.duration,
    availableAudioTracks: [],
    availableSubtitleTracks: [],
    availableVideoTracks: [],
  });

  // playToEnd carries no payload, so it can't be a useEvent value; a flag cleared on replay.
  const [ended, setEnded] = useState(false);
  useEventListener(player, "playToEnd", () => setEnded(true));
  useEventListener(player, "playingChange", ({ isPlaying }) => {
    if (isPlaying) setEnded(false);
  });

  return {
    status,
    error: error ? new PlaybackError(error.message) : undefined,
    playing: isPlaying,
    buffering: status === "loading",
    ended,
    currentTime,
    bufferedPosition,
    duration,
  };
}
