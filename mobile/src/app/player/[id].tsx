import { useEventListener } from "expo";
import { Stack, useLocalSearchParams } from "expo-router";
import { useVideoPlayer, VideoView } from "expo-video";
import { useCallback, useEffect, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { MediaKind, streamUrl, type Media } from "@/api/media";
import { ErrorView } from "@/components/ErrorView";
import { Loading } from "@/components/Loading";
import { PlayerControls } from "@/components/PlayerControls";
import { useAsync } from "@/hooks/useAsync";
import { PlaybackError, usePlayerStatus } from "@/hooks/usePlayerStatus";
import { formatDuration } from "@/lib/format";
import { loadWithProgress, resumeAt } from "@/progress/resume";
import { useProgressSync } from "@/progress/useProgressSync";

// The player screen fetches the media first: a bad id becomes a "Not found" from the API
// instead of a stream the native player can't explain. It also says where to resume.
export default function PlayerScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const fetcher = useCallback((signal: AbortSignal) => loadWithProgress(id, signal), [id]);
  const { state, refetch } = useAsync(fetcher);

  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <ErrorView error={state.error} onRetry={refetch} />;
  if (state.data.kind !== MediaKind.Video) return <ErrorView error={new PlaybackError(`${state.data.title} is not a video.`)} />;
  return <Player media={state.data} />;
}

function Player({ media }: { media: Media }) {
  const source = { uri: streamUrl(media.id), metadata: { title: media.title } };

  // The player is a native object (AVPlayer / ExoPlayer) outside the React tree. The hook
  // releases it on unmount, which closes its connections: leaving the screen stops the 206.
  // Defaults this phase relies on: keepScreenOnWhilePlaying is true, and with
  // staysActiveInBackground false the player pauses when the app is backgrounded.
  const player = useVideoPlayer(source, (p) => {
    p.timeUpdateEventInterval = 0.5; // 0 means no timeUpdate events at all
    p.currentTime = resumeAt(media); // seek before play, so the start isn't heard first
    p.play();
  });
  const s = usePlayerStatus(player);

  // Progress: throttled saves while playing; immediate ones on pause, seek, leaving the screen
  // and backgrounding (the last two inside useProgressSync). Never while loading or buffering:
  // the position then is where the player is stuck, not where you are.
  const sync = useProgressSync(media.id);
  const position = (seconds: number) => ({
    positionMs: Math.round(seconds * 1000),
    durationMs: Math.round(player.duration * 1000),
  });
  useEventListener(player, "timeUpdate", ({ currentTime }) => {
    if (player.playing && player.status === "readyToPlay" && player.duration > 0) sync.save(position(currentTime));
  });
  useEventListener(player, "playingChange", ({ isPlaying }) => {
    if (!isPlaying && player.duration > 0) sync.save(position(player.currentTime), { force: true });
  });
  const onSeek = (to: number) => {
    if (player.duration > 0) sync.save(position(to), { force: true });
  };
  const videoRef = useRef<VideoView>(null);

  // Fullscreen is the OS's player, and on iOS it keeps whatever nativeControls the inline view had:
  // with them off, fullscreen has no controls at all. So turn them on first, enter fullscreen once
  // that has rendered, and turn them off again on exit, when our glass controls are back.
  const [fullscreen, setFullscreen] = useState(false);
  useEffect(() => {
    if (!fullscreen) return;
    videoRef.current?.enterFullscreen().catch(() => setFullscreen(false));
  }, [fullscreen]);

  const retry = () => {
    void player.replaceAsync(source).then(() => player.play());
  };

  return (
    <View style={styles.screen}>
      <Stack.Screen options={{ title: media.title }} />

      {s.error ? (
        <ErrorView error={s.error} onRetry={retry} />
      ) : (
        <View style={styles.videoBox}>
          {/* Our glass controls inline; the OS's own controls in fullscreen. */}
          <VideoView
            ref={videoRef}
            player={player}
            style={styles.video}
            nativeControls={fullscreen}
            fullscreenOptions={{ enable: true }}
            onFullscreenExit={() => setFullscreen(false)}
          />
          {!fullscreen && <PlayerControls player={player} status={s} onFullscreen={() => setFullscreen(true)} onSeek={onSeek} />}
        </View>
      )}

      {/* Raw player state, for exercises 2-4: watch it change while you seek and starve it. */}
      <Text style={styles.debug}>
        {s.status}
        {s.playing ? " · playing" : " · paused"}
        {s.ended ? " · ended" : ""} · {formatDuration(s.currentTime)} / {formatDuration(s.duration)} · buffered to{" "}
        {formatDuration(s.bufferedPosition)}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, justifyContent: "center" },
  videoBox: { width: "100%", aspectRatio: 16 / 9, backgroundColor: "#000" },
  video: { flex: 1 },
  debug: { color: "#888", fontSize: 12, textAlign: "center", padding: 12, fontVariant: ["tabular-nums"] },
});
