import { Image } from "expo-image";
import { StyleSheet } from "react-native";
import {
  GestureDetector,
  useExclusiveGestures,
  usePanGesture,
  usePinchGesture,
  useSimultaneousGestures,
  useTapGesture,
} from "react-native-gesture-handler";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { scheduleOnRN } from "react-native-worklets";

const MAX_SCALE = 5;

type Props = {
  uri: string;
  width: number;
  height: number;
  zoomed: boolean; // the list's view of this page: while zoomed, a drag pans instead of turning
  onZoomChange: (zoomed: boolean) => void;
  onTap: () => void;
  label: string;
};

// ComicPage is one page: pinch to zoom, drag to pan while zoomed, double-tap to reset, single tap
// to show or hide the reader's controls. The gestures run on the UI thread (Reanimated), so
// zooming stays smooth whatever the JS thread is doing.
export function ComicPage({ uri, width, height, zoomed, onZoomChange, onTap, label }: Props) {
  const scale = useSharedValue(1);
  const startScale = useSharedValue(1);
  const x = useSharedValue(0);
  const y = useSharedValue(0);
  const startX = useSharedValue(0);
  const startY = useSharedValue(0);

  const pinch = usePinchGesture({
    onUpdate: (e) => {
      "worklet";
      scale.set(Math.min(MAX_SCALE, Math.max(1, startScale.get() * e.scale)));
    },
    onDeactivate: () => {
      "worklet";
      startScale.set(scale.get());
      const z = scale.get() > 1.01;
      if (!z) {
        x.set(withTiming(0));
        y.set(withTiming(0));
        startX.set(0);
        startY.set(0);
      }
      scheduleOnRN(onZoomChange, z);
    },
  });

  // Enabled only while zoomed: otherwise this pan would swallow the list's swipe.
  const pan = usePanGesture({
    enabled: zoomed,
    onUpdate: (e) => {
      "worklet";
      // Keep the page's edges from leaving the screen.
      const maxX = ((scale.get() - 1) * width) / 2;
      const maxY = ((scale.get() - 1) * height) / 2;
      x.set(Math.min(maxX, Math.max(-maxX, startX.get() + e.translationX)));
      y.set(Math.min(maxY, Math.max(-maxY, startY.get() + e.translationY)));
    },
    onDeactivate: () => {
      "worklet";
      startX.set(x.get());
      startY.set(y.get());
    },
  });

  const doubleTap = useTapGesture({
    numberOfTaps: 2,
    onActivate: () => {
      "worklet";
      scale.set(withTiming(1));
      x.set(withTiming(0));
      y.set(withTiming(0));
      startScale.set(1);
      startX.set(0);
      startY.set(0);
      scheduleOnRN(onZoomChange, false);
    },
  });
  const singleTap = useTapGesture({
    onActivate: () => {
      "worklet";
      scheduleOnRN(onTap);
    },
  });

  // A single tap waits until it's clear the tap isn't the first half of a double tap.
  const taps = useExclusiveGestures(doubleTap, singleTap);
  const gesture = useSimultaneousGestures(pinch, pan, taps);

  const style = useAnimatedStyle(() => ({
    transform: [{ translateX: x.get() }, { translateY: y.get() }, { scale: scale.get() }],
  }));

  return (
    <GestureDetector gesture={gesture}>
      <Animated.View style={[{ width, height }, styles.page]} accessible accessibilityLabel={label}>
        <Animated.View style={[StyleSheet.absoluteFill, style]}>
          {/* expo-image: disk cache that honours Cache-Control, and a short fade instead of a pop. */}
          <Image source={{ uri }} style={StyleSheet.absoluteFill} contentFit="contain" transition={120} recyclingKey={uri} />
        </Animated.View>
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  page: { overflow: "hidden", backgroundColor: "#000" },
});
