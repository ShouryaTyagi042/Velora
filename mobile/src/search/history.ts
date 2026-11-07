import AsyncStorage from "@react-native-async-storage/async-storage";
import { useEffect, useSyncExternalStore } from "react";

// Recent searches, newest first, on this device only (the server never sees them). A search is
// remembered when it is submitted or when one of its results is opened: what was looked for and
// found, not every half-typed word. Signing out forgets them.

const KEY = "search:history";
const MAX = 12;

let entries: string[] = [];
let loaded = false;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

async function load() {
  if (loaded) return;
  loaded = true;
  try {
    const raw = await AsyncStorage.getItem(KEY);
    if (raw) entries = JSON.parse(raw) as string[];
    emit();
  } catch {
    // Storage unavailable: start with no history.
  }
}

function persist() {
  emit();
  AsyncStorage.setItem(KEY, JSON.stringify(entries)).catch(() => {});
}

// remember puts q at the top. The same search typed differently ("matrix", "Matrix ") is one entry.
export function remember(q: string) {
  const query = q.trim();
  if (!query) return;
  const same = query.toLocaleLowerCase();
  entries = [query, ...entries.filter((e) => e.toLocaleLowerCase() !== same)].slice(0, MAX);
  persist();
}

export function forget(q: string) {
  entries = entries.filter((e) => e !== q);
  persist();
}

export function clearHistory() {
  entries = [];
  persist();
}

// useSearchHistory is the list, loaded from storage the first time a screen asks.
export function useSearchHistory(): string[] {
  useEffect(() => {
    void load();
  }, []);
  return useSyncExternalStore(subscribe, () => entries);
}
