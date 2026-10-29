import AsyncStorage from "@react-native-async-storage/async-storage";

import type { ProgressBody } from "@/api/progress";

// The device's own copy of the last known progress per item, written before every send. If the
// send fails (Wi-Fi drops, the app is killed), the position survives here and is pushed on the
// next launch. A two-key outbox: the seed of every offline-sync design.

export type LocalProgress = {
  body: ProgressBody;
  clientTs: string; // RFC 3339, this device's clock
  synced: boolean; // the server has acknowledged this exact save
};

const PREFIX = "progress:";

export async function loadLocal(id: string): Promise<LocalProgress | null> {
  try {
    const raw = await AsyncStorage.getItem(PREFIX + id);
    return raw ? (JSON.parse(raw) as LocalProgress) : null;
  } catch {
    return null; // storage unavailable: behave as if there's nothing local
  }
}

export async function saveLocal(id: string, p: LocalProgress): Promise<void> {
  try {
    await AsyncStorage.setItem(PREFIX + id, JSON.stringify(p));
  } catch {
    // Storage full or unavailable: the network send still happens.
  }
}

// unsynced returns every local save the server hasn't acknowledged, by media id.
export async function unsynced(): Promise<[string, LocalProgress][]> {
  try {
    const keys = (await AsyncStorage.getAllKeys()).filter((k) => k.startsWith(PREFIX));
    const pairs = await AsyncStorage.multiGet(keys);
    const out: [string, LocalProgress][] = [];
    for (const [key, raw] of pairs) {
      if (!raw) continue;
      const p = JSON.parse(raw) as LocalProgress;
      if (!p.synced) out.push([key.slice(PREFIX.length), p]);
    }
    return out;
  } catch {
    return [];
  }
}

// clearLocal forgets every local save: on sign-out, so the next person to sign in on this phone
// never has someone else's positions pushed as theirs.
export async function clearLocal(): Promise<void> {
  try {
    const keys = (await AsyncStorage.getAllKeys()).filter((k) => k.startsWith(PREFIX));
    await AsyncStorage.multiRemove(keys);
  } catch {
    // Storage unavailable: nothing we can remove.
  }
}
