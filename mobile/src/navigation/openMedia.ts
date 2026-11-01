import { router } from "expo-router";

import { MediaKind, type Media } from "@/api/media";

// openMedia is what tapping a card does. A comic opens straight in the reader, at the page you
// were on (the reader resumes from the saved progress), with no Details screen in between: there
// is nothing on it a comic needs. A video opens Details: Play / Resume, its actors, its series.
export function openMedia(m: Media) {
  if (m.kind === MediaKind.Comic) {
    router.push({ pathname: "/reader/[id]", params: { id: m.id } });
  } else {
    router.push({ pathname: "/media/[id]", params: { id: m.id } });
  }
}
