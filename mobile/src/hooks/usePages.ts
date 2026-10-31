import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { isAbort } from "@/api/client";
import type { Page } from "@/api/media";
import { useAsync, type Async } from "./useAsync";

// What has happened since a first page arrived: the pages loaded after it, and whether the next
// is loading or failed. It records which first page it continues, so when a new first page
// arrives (a new list, or a refresh) the old record simply stops applying; nothing has to reset it.
type More<T> = { first: Page<T>; items: T[]; next: string | null; loading: boolean; error: Error | null };

// usePages is useAsync for a list the server sends a page at a time: the first page loads like
// any fetch, and loadMore (wired to FlatList's onEndReached) appends the next.
//
// fetchPage must be stable unless its inputs change (wrap it in useCallback), as with useAsync:
// a new fetchPage is a new list, so it starts again from the first page. So does refetch.
export function usePages<T>(fetchPage: (after: string | undefined, signal: AbortSignal) => Promise<Page<T>>) {
  const first = useCallback((signal: AbortSignal) => fetchPage(undefined, signal), [fetchPage]);
  const { state: firstState, refreshing, refetch } = useAsync(first);
  const page = firstState.status === "ok" ? firstState.data : null;

  const [more, setMore] = useState<More<T> | null>(null);
  const inFlight = useRef<AbortController | null>(null);

  // A new first page makes a page still loading for the old one useless: cancel it.
  useEffect(() => {
    return () => {
      inFlight.current?.abort();
      inFlight.current = null;
    };
  }, [page]);

  const current = page && more?.first === page ? more : null;
  const next = current ? current.next : (page?.nextCursor ?? null);

  const loadMore = useCallback(() => {
    // One page at a time: onEndReached can fire several times while a page is on its way.
    if (!page || !next || inFlight.current) return;
    const ac = new AbortController();
    inFlight.current = ac;
    const base = { first: page, items: current?.items ?? [], next };
    setMore({ ...base, loading: true, error: null });
    fetchPage(next, ac.signal).then(
      (p) => {
        if (ac.signal.aborted) return;
        inFlight.current = null;
        setMore({ first: page, items: [...base.items, ...p.items], next: p.nextCursor, loading: false, error: null });
      },
      (err: unknown) => {
        if (ac.signal.aborted || isAbort(err)) return;
        inFlight.current = null;
        setMore({ ...base, loading: false, error: err instanceof Error ? err : new Error(String(err)) });
      },
    );
  }, [fetchPage, page, current, next]);

  // The same array until a page arrives, so FlatList doesn't re-render its rows on every render.
  const extra = current?.items;
  const items = useMemo(() => (page ? (extra ? [...page.items, ...extra] : page.items) : []), [page, extra]);
  const state: Async<T[]> = firstState.status === "ok" ? { status: "ok", data: items } : firstState;

  return {
    state,
    refreshing,
    refetch,
    loadMore,
    loadingMore: current?.loading ?? false,
    moreError: current?.error ?? null,
    hasMore: next !== null,
  };
}
