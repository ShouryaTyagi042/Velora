import { Platform } from "react-native";

// Options for each tab's own stack. On iOS the header is transparent so content scrolls under
// it (iOS 26 draws it as glass, and softens content at its edge); scroll views pad themselves
// with contentInsetAdjustmentBehavior="automatic". Android keeps a solid header: no glass there.
export const tabStackOptions = {
  headerLargeTitle: true,
  headerTransparent: Platform.OS === "ios",
  headerBackButtonDisplayMode: "minimal",
} as const;
