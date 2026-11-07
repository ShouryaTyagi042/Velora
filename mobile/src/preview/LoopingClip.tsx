import { useVideoPlayer, VideoView, type VideoPlayer } from "expo-video";
import { useMemo, useState } from "react";
import { Pressable, StyleSheet, type StyleProp, type ViewStyle } from "react-native";

import { authHeaders } from "@/api/client";
import { previewUrl, shortUrl, type Media } from "@/api/media";
import { Icon } from "@/components/Icon";
import { onMedia, radius, space, touch } from "@/theme";

// VideoTeaser plays a video's clip over its still: the Shorts clip if the server has made one,
// else the long-press preview; looping, muted until you tap the speaker. The still stays until the
// clip's first frame is drawn. Nothing for a video with neither. Surprise me and the details page.
// soundStyle moves the speaker button, where something covers the clip's bottom corner.
export function VideoTeaser({ media, soundStyle }: { media: Media; soundStyle?: StyleProp<ViewStyle> }) {
  // Only the Shorts clip has a soundtrack: the long-press preview is cut silent.
  if (media.video?.short) return <LoopingClip uri={shortUrl(media)} hasSound soundStyle={soundStyle} />;
  if (media.video?.preview) return <LoopingClip uri={previewUrl(media)} hasSound={false} />;
  return null;
}

// Sound on or off is remembered while the app runs: the next clip, wherever it plays, sounds the
// way you left it.
let soundOn = false;

// The player is a native object: setting its properties is a command, not a React mutation.
function applySound(p: VideoPlayer, on: boolean) {
  p.muted = !on;
  // Silent, it mixes with music playing elsewhere; with sound, it takes the audio like any video.
  p.audioMixingMode = on ? "auto" : "mixWithOthers";
}

function LoopingClip({ uri, hasSound, soundStyle }: { uri: string; hasSound: boolean; soundStyle?: StyleProp<ViewStyle> }) {
  const source = useMemo(() => ({ uri, headers: authHeaders() }), [uri]);
  const player = useVideoPlayer(source, (p) => {
    p.loop = true;
    applySound(p, hasSound && soundOn);
    p.play();
  });
  const [drawn, setDrawn] = useState(false);
  const [sound, setSound] = useState(hasSound && soundOn);
  const toggle = () => {
    soundOn = !sound;
    applySound(player, soundOn);
    setSound(soundOn);
  };
  return (
    <>
      <VideoView
        player={player}
        style={[StyleSheet.absoluteFill, { opacity: drawn ? 1 : 0 }]}
        contentFit="cover"
        nativeControls={false}
        allowsPictureInPicture={false}
        onFirstFrameRender={() => setDrawn(true)}
        pointerEvents="none"
      />
      {hasSound && (
        <Pressable
          onPress={toggle}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={sound ? "Turn sound off" : "Turn sound on"}
          style={({ pressed }) => [styles.sound, soundStyle, pressed && styles.soundPressed]}
        >
          <Icon sf={sound ? "speaker.wave.2.fill" : "speaker.slash.fill"} md={sound ? "volume_up" : "volume_off"} size={16} color={onMedia.text} />
        </Pressable>
      )}
    </>
  );
}

const styles = StyleSheet.create({
  sound: {
    position: "absolute",
    right: space.sm,
    bottom: space.sm,
    width: touch.min - 6,
    height: touch.min - 6,
    borderRadius: radius.pill,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: onMedia.scrim,
  },
  soundPressed: { opacity: 0.6 },
});
