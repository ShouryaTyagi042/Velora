import { useEffect, useState } from "react";
import { AppState } from "react-native";

import { ProgressSync } from "./sync";

// useProgressSync returns the ProgressSync for one item, and flushes it when the app goes to the
// background and when the screen goes away: the two moments a throttled save would otherwise lose.
export function useProgressSync(id: string): ProgressSync {
  const [sync] = useState(() => new ProgressSync(id));
  useEffect(() => {
    const sub = AppState.addEventListener("change", (state) => {
      if (state !== "active") sync.flush();
    });
    return () => {
      sub.remove();
      sync.flush();
    };
  }, [sync]);
  return sync;
}
