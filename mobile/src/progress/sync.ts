import { putProgress, type ProgressBody } from "@/api/progress";
import { saveLocal, unsynced, type LocalProgress } from "./store";

// SAVE_EVERY is the throttle: at most one save per 10 s while playing. Losing 10 s of position
// is imperceptible; a save on every timeUpdate (twice a second) would flood the server.
export const SAVE_EVERY_MS = 10_000;

// ProgressSync saves one item's progress: local first, then the server, with at most one request
// in flight. A save that comes in while one is in flight replaces any waiting save and is sent
// when the first returns, so requests can't overtake each other.
export class ProgressSync {
  private lastSent = 0;
  private inFlight = false;
  private waiting: LocalProgress | null = null;
  private latest: ProgressBody | null = null;

  constructor(private readonly id: string) {}

  // save records body. Unless force is set, it is sent only if SAVE_EVERY_MS has passed since
  // the last send (a throttle, not a debounce: playback is never quiet long enough to debounce).
  // Force it on pause, seek, leaving the screen and backgrounding.
  save(body: ProgressBody, { force = false } = {}): void {
    this.latest = body;
    const now = Date.now();
    if (!force && now - this.lastSent < SAVE_EVERY_MS) return;
    this.lastSent = now;
    const p: LocalProgress = { body, clientTs: new Date(now).toISOString(), synced: false };
    void saveLocal(this.id, p);
    this.send(p);
  }

  // flush sends the most recent body now, if there is one. For unmount and backgrounding.
  flush(): void {
    if (this.latest) this.save(this.latest, { force: true });
  }

  private send(p: LocalProgress): void {
    if (this.inFlight) {
      this.waiting = p; // only the newest waiting save matters
      return;
    }
    this.inFlight = true;
    putProgress(this.id, p.body, p.clientTs).then(
      () => saveLocal(this.id, { ...p, synced: true }),
      () => {}, // stays unsynced locally; pushed on the next launch
    ).finally(() => {
      this.inFlight = false;
      const next = this.waiting;
      this.waiting = null;
      if (next) this.send(next);
    });
  }
}

// reconcile pushes every local save the server never acknowledged, e.g. from a session that
// ended offline. Safe to repeat: the server keeps whichever save is newest.
export async function reconcile(): Promise<void> {
  for (const [id, p] of await unsynced()) {
    try {
      await putProgress(id, p.body, p.clientTs);
      await saveLocal(id, { ...p, synced: true });
    } catch {
      // Still offline, or the item is gone (404): try again next launch.
    }
  }
}
