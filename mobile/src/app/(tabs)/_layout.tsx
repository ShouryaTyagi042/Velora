import { NativeTabs } from "expo-router/native-tabs";

import { PreviewProvider } from "@/preview/PreviewProvider";
import { palette } from "@/theme";

// Native tabs, five of them (a sixth would put the rest behind a "More" tab on iPhone):
// Home, the browse-everything page, which also leads to Comics and Surprise me; Shorts, a
// vertical feed shuffling videos and anime; Actors, the movie library by who's in it; Anime, by
// series; and Search. The system's own tab bar: Liquid Glass on iOS 26 (shrinking while you scroll
// down), Material on Android. Velora blue marks the selected tab.
export default function TabsLayout() {
  // PreviewProvider: the one long-press preview player, shared by every card in every tab.
  return (
    <PreviewProvider>
      <NativeTabs minimizeBehavior="onScrollDown" tintColor={palette.dark.primaryText} blurEffect="systemChromeMaterialDark">
        <NativeTabs.Trigger name="(home)">
          <NativeTabs.Trigger.Label>Home</NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={{ default: "house", selected: "house.fill" }} md="home" />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="(shorts)">
          <NativeTabs.Trigger.Label>Shorts</NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={{ default: "play.rectangle.on.rectangle", selected: "play.rectangle.on.rectangle.fill" }} md="smart_display" />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="(actors)">
          <NativeTabs.Trigger.Label>Actors</NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={{ default: "person.2", selected: "person.2.fill" }} md="groups" />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="(anime)">
          <NativeTabs.Trigger.Label>Anime</NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={{ default: "sparkles.tv", selected: "sparkles.tv.fill" }} md="live_tv" />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="(search)" role="search">
          <NativeTabs.Trigger.Label>Search</NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf="magnifyingglass" md="search" />
        </NativeTabs.Trigger>
      </NativeTabs>
    </PreviewProvider>
  );
}
