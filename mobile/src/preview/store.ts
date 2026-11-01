import { useSyncExternalStore } from "react";

import type { Media } from "@/api/media";

// Which card is previewing, if any: one at a time, since there is one shared player.
// A tiny external store rather than React state at the top, so a long-press re-renders the two
// cards that changed (the old one and the new one), not every card on the screen: each card
// subscribes to "am I the one?", and useSyncExternalStore re-renders it only when that answer flips.

// seq makes every start a new value, so long-pressing the card that just finished plays it again.
type Current = { media: Media; seq: number } | null;

let current: Current = null;
let seq = 0;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// startPreview makes m the previewing card, taking the player from any other.
export function startPreview(m: Media) {
  current = { media: m, seq: ++seq };
  emit();
}

// stopPreview ends the preview. With an id, only if that card is still the one previewing: a card
// that unmounts or loses focus mustn't stop a preview another card has since started.
export function stopPreview(id?: string) {
  if (!current || (id !== undefined && current.media.id !== id)) return;
  current = null;
  emit();
}

// useCurrentPreview is what the shared player follows.
export function useCurrentPreview(): Current {
  return useSyncExternalStore(subscribe, () => current);
}

// useIsPreviewing is what a card asks: am I the one previewing?
export function useIsPreviewing(id: string): boolean {
  return useSyncExternalStore(subscribe, () => current?.media.id === id);
}
