import { useEffect, useState } from "react";

import { isAbort } from "@/api/client";

// The states of one fetch. A union, so "data and an error at once" can't be represented.
export type Async<T> =
  | { status: "loading" }
  | { status: "ok"; data: T }
  | { status: "error"; error: Error };

type Settled<T> = {
  fetcher: (signal: AbortSignal) => Promise<T>; // which request this result belongs to
  attempt: number;
  result: Exclude<Async<T>, { status: "loading" }>;
};

// useAsync runs fetcher, and again whenever fetcher changes or refetch() is called.
//
// fetcher must be stable between renders unless its inputs change: wrap it in useCallback
// with its inputs as deps (e.g. [kind, actor]). That dep list is what makes a new filter
// trigger a new request, and the hooks lint rule checks it is complete.
//
// The previous request is aborted when a new one starts or the screen unmounts, so a
// slow old response can never overwrite a newer one.
export function useAsync<T>(fetcher: (signal: AbortSignal) => Promise<T>) {
  const [attempt, setAttempt] = useState(0);
  const [settled, setSettled] = useState<Settled<T> | null>(null);

  useEffect(() => {
    const ac = new AbortController();
    fetcher(ac.signal).then(
      (data) => {
        if (!ac.signal.aborted) setSettled({ fetcher, attempt, result: { status: "ok", data } });
      },
      (err: unknown) => {
        if (ac.signal.aborted || isAbort(err)) return;
        const error = err instanceof Error ? err : new Error(String(err));
        setSettled({ fetcher, attempt, result: { status: "error", error } });
      },
    );
    return () => ac.abort();
  }, [fetcher, attempt]);

  // Loading is derived, not stored: the latest result is for an older request.
  const current = settled?.fetcher === fetcher && settled.attempt === attempt;
  const sameRequest = settled?.fetcher === fetcher;

  let state: Async<T>;
  if (current) {
    state = settled.result;
  } else if (sameRequest && settled.result.status === "ok") {
    state = settled.result; // refetching: keep showing the old data under the spinner
  } else {
    state = { status: "loading" };
  }

  return {
    state,
    // true while a pull-to-refresh is running over data already on screen
    refreshing: sameRequest && !current && state.status === "ok",
    refetch: () => setAttempt((n) => n + 1),
  };
}
