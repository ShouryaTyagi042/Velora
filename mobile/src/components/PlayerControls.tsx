import Slider from "@react-native-community/slider";
import { useEventListener } from "expo";
import { GlassContainer } from "expo-glass-effect";
import type { VideoPlayer } from "expo-video";
import { useEffect, useState } from "react";
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { useAnimatedStyle, useReducedMotion, withTiming } from "react-native-reanimated";

import type { PlayerStatus } from "@/hooks/usePlayerStatus";
import { formatDuration } from "@/lib/format";
import { Icon } from "./Icon";
import type { AndroidSymbol, SFSymbol } from "expo-symbols";
import { Surface } from "./Surface";

const HIDE_AFTER_MS = 3000;
const SKIP_SECONDS = 10;
const FADE_MS = 250;
const FG = "#fff"; // controls sit over video, which is dark more often than not; tokens in phase 10b

type Props = {
  player: VideoPlayer;
  status: PlayerStatus;
  onFullscreen: () => void;
  onSeek?: (to: number) => void; // after every seek or skip, with the target in seconds
};

// PlayerControls is the overlay on the video: ±10 s and play/pause in the middle, time, seek bar and
// fullscreen at the bottom, all on glass. Tap the video to show or hide them; while playing they
// hide after 3 s without a touch. Paused or ended, they stay.
export function PlayerControls({ player, status: s, onFullscreen, onSeek }: Props) {
  const reduceMotion = useReducedMotion();

  const [visible, setVisible] = useState(true);
  const [touchedAt, setTouchedAt] = useState(0); // any interaction restarts the hide timer
  const [scrubbing, setScrubbing] = useState(false);
  const [scrubTime, setScrubTime] = useState(0);
  // After a seek the player reports the old time until its next timeUpdate; keep showing the
  // target meanwhile so the thumb doesn't jump back and forward again.
  const [seekTarget, setSeekTarget] = useState<number | null>(null);

  const shown = visible || !s.playing || s.ended;
  const touch = () => {
    setVisible(true);
    setTouchedAt(Date.now());
  };

  useEffect(() => {
    if (!shown || !s.playing || scrubbing) return;
    const t = setTimeout(() => setVisible(false), HIDE_AFTER_MS);
    return () => clearTimeout(t);
  }, [shown, s.playing, scrubbing, touchedAt]);

  // The first timeUpdate near the target means the seek landed.
  useEventListener(player, "timeUpdate", ({ currentTime }) => {
    setSeekTarget((t) => (t !== null && Math.abs(currentTime - t) < 1 ? null : t));
  });
  useEffect(() => {
    if (seekTarget === null) return;
    const t = setTimeout(() => setSeekTarget(null), 2000); // a seek that never lands
    return () => clearTimeout(t);
  }, [seekTarget]);

  // Fade the contents, not the glass: glass doesn't render below opacity 1, so Surface turns its
  // glass off natively (visible={shown}) while this wrapper fades what's on it.
  const fade = useAnimatedStyle(() => ({
    opacity: withTiming(shown ? 1 : 0, { duration: reduceMotion ? 0 : FADE_MS }),
  }));

  const time = scrubbing ? scrubTime : (seekTarget ?? s.currentTime);
  const canSeek = s.duration > 0;

  const playPause = () => {
    touch();
    if (s.ended) player.replay();
    else if (s.playing) player.pause();
    else player.play();
  };
  const skip = (by: number) => {
    touch();
    const to = Math.min(Math.max(0, time + by), s.duration);
    setSeekTarget(to);
    player.seekBy(by);
    onSeek?.(to);
  };
  const seekTo = (to: number) => {
    setScrubbing(false);
    setSeekTarget(to);
    // A new Range request: watch for the 206 in the server log. seekBy rather than assigning
    // currentTime, which the React Compiler rejects as mutating a prop.
    player.seekBy(to - player.currentTime);
    onSeek?.(to);
    touch();
  };

  const center: { sf: SFSymbol; md: AndroidSymbol; label: string } = s.ended
    ? { sf: "arrow.counterclockwise", md: "replay", label: "Replay" }
    : s.playing
      ? { sf: "pause.fill", md: "pause", label: "Pause" }
      : { sf: "play.fill", md: "play_arrow", label: "Play" };

  return (
    // The background tap toggles the controls; buttons on top take their own taps first.
    <Pressable
      style={StyleSheet.absoluteFill}
      onPress={() => (shown && s.playing ? setVisible(false) : touch())}
      accessible={false}
    >
      {s.buffering && !shown && (
        <View style={styles.center} pointerEvents="none">
          <ActivityIndicator size="large" color={FG} />
        </View>
      )}

      <Animated.View style={[StyleSheet.absoluteFill, fade]} pointerEvents={shown ? "box-none" : "none"}>
        <View style={styles.center} pointerEvents="box-none">
          {/* One container: the three buttons are one piece of glass and merge as one material. */}
          <GlassContainer spacing={24} style={styles.centerRow}>
            <RoundButton size={48} visible={shown} label={`Back ${SKIP_SECONDS} seconds`} onPress={() => skip(-SKIP_SECONDS)}>
              <Icon sf="gobackward.10" md="replay_10" size={22} color={FG} />
            </RoundButton>
            <RoundButton size={68} visible={shown} label={center.label} onPress={playPause}>
              {s.buffering && s.playing ? (
                <ActivityIndicator color={FG} />
              ) : (
                <Icon sf={center.sf} md={center.md} size={30} color={FG} />
              )}
            </RoundButton>
            <RoundButton size={48} visible={shown} label={`Forward ${SKIP_SECONDS} seconds`} onPress={() => skip(SKIP_SECONDS)}>
              <Icon sf="goforward.10" md="forward_10" size={22} color={FG} />
            </RoundButton>
          </GlassContainer>
        </View>

        <Surface role="bar" visible={shown} colorScheme="dark" style={styles.bar}>
          <Text style={styles.time}>{formatDuration(time)}</Text>
          <Slider
            style={styles.slider}
            minimumValue={0}
            maximumValue={canSeek ? s.duration : 1}
            value={canSeek ? time : 0}
            disabled={!canSeek}
            minimumTrackTintColor={FG}
            maximumTrackTintColor="rgba(255, 255, 255, 0.35)"
            thumbTintColor={FG}
            onSlidingStart={(v) => {
              setScrubbing(true);
              setScrubTime(v);
            }}
            // Only the drag's end seeks: seeking on every move would open a Range request per frame.
            onValueChange={(v) => scrubbing && setScrubTime(v)}
            onSlidingComplete={seekTo}
            accessibilityLabel="Playback position"
          />
          <Text style={styles.time}>{formatDuration(s.duration)}</Text>
          <Pressable
            onPress={() => {
              touch();
              onFullscreen();
            }}
            hitSlop={8}
            accessibilityRole="button"
            accessibilityLabel="Full screen"
            style={styles.barButton}
          >
            <Icon sf="arrow.up.left.and.arrow.down.right" md="fullscreen" size={18} color={FG} />
          </Pressable>
        </Surface>
      </Animated.View>
    </Pressable>
  );
}

type RoundButtonProps = { size: number; visible: boolean; label: string; onPress: () => void; children: React.ReactNode };

function RoundButton({ size, visible, label, onPress, children }: RoundButtonProps) {
  return (
    <Pressable onPress={onPress} accessibilityRole="button" accessibilityLabel={label}>
      <Surface
        role="control"
        visible={visible}
        colorScheme="dark"
        style={[styles.round, { width: size, height: size, borderRadius: size / 2 }]}
      >
        {children}
      </Surface>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  center: { ...StyleSheet.absoluteFill, alignItems: "center", justifyContent: "center" },
  centerRow: { flexDirection: "row", alignItems: "center", gap: 24 },
  round: { alignItems: "center", justifyContent: "center", overflow: "hidden" },
  bar: {
    position: "absolute",
    left: 8,
    right: 8,
    bottom: 8,
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    paddingHorizontal: 12,
    minHeight: 44,
    borderRadius: 22,
    overflow: "hidden",
  },
  time: { color: FG, fontSize: 12, fontVariant: ["tabular-nums"] },
  slider: { flex: 1, height: 44 },
  barButton: { width: 32, height: 44, alignItems: "center", justifyContent: "center" },
});
