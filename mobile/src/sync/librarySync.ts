import { useSyncExternalStore } from "react";

import { getJob, scanLibrary, type ScanResult } from "@/api/media";
import { libraryChanged } from "@/hooks/useLibraryChanges";

// Library sync: ask the server to rescan its folder, so files copied in since show up and deleted
// ones drop out. The scan runs on the server; the phone only polls the job. The state lives here,
// outside any screen, so a sync keeps going (and keeps being shown) while you browse elsewhere.

export type SyncState =
  | { status: "idle" }
  | { status: "syncing"; done?: number; total?: number }
  | { status: "done"; result?: ScanResult; at: number }
  | { status: "failed"; error: string; at: number };

const POLL_EVERY = 1000;

let state: SyncState = { status: "idle" };
const listeners = new Set<() => void>();

function set(next: SyncState) {
  state = next;
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useLibrarySync(): SyncState {
  return useSyncExternalStore(subscribe, () => state);
}

// syncLibrary starts a sync, unless one is already running. When it ends, every list refetches
// the next time it's shown.
export async function syncLibrary() {
  if (state.status === "syncing") return;
  set({ status: "syncing" });
  try {
    let job = await scanLibrary();
    while (job.status === "running") {
      if (job.progress) set({ status: "syncing", done: job.progress.done, total: job.progress.total });
      await new Promise((r) => setTimeout(r, POLL_EVERY));
      job = await getJob(job.id);
    }
    if (job.status === "done") set({ status: "done", result: job.result, at: Date.now() });
    else set({ status: "failed", error: job.error || `The scan was ${job.status}.`, at: Date.now() });
  } catch (e) {
    set({ status: "failed", error: e instanceof Error ? e.message : String(e), at: Date.now() });
  } finally {
    libraryChanged();
  }
}
