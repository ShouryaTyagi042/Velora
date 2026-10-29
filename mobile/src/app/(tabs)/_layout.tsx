import { NativeTabs } from "expo-router/native-tabs";

// Native tabs: Home, then one tab per kind (each with its own layout and filters), then Search
// across both. The system's own tab bar, which is Liquid Glass on iOS 26 (and shrinks while you
// scroll down) and a Material bar on Android. Search is a search-role tab: on iOS 26 its field
// lives in the tab bar itself. Icons: SF Symbols on iOS, Material Symbols on Android.
export default function TabsLayout() {
  return (
    <NativeTabs minimizeBehavior="onScrollDown">
      <NativeTabs.Trigger name="(home)">
        <NativeTabs.Trigger.Label>Home</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon sf={{ default: "house", selected: "house.fill" }} md="home" />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="(videos)">
        <NativeTabs.Trigger.Label>Videos</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon sf={{ default: "film", selected: "film.fill" }} md="movie" />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="(comics)">
        <NativeTabs.Trigger.Label>Comics</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon sf={{ default: "book", selected: "book.fill" }} md="menu_book" />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="(search)" role="search">
        <NativeTabs.Trigger.Label>Search</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon sf="magnifyingglass" md="search" />
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
