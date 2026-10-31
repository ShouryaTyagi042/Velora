import { useEffect, useRef, useState } from "react";

import { getJob, scanLibrary } from "@/api/media";

// POLL_EVERY is how often a running scan is checked. A scan of a few hundred files takes well
// under a second; 10,000 files take about 8 s.
const POLL_EVERY = 1000;

// useRescan is pull-to-refresh for a library screen: rescan the server's folder (so newly copied
// files appear), wait for the scan to finish, then refetch the screen's data.
//
// The server answers the scan request at once with a job and scans in the background; this polls
// the job until it's no longer running. Leaving the screen stops the polling, not the scan. A
// failed scan, or a lost one (the server restarted), still refetches what's there.
export function useRescan(refetch: () => void) {
  const [scanning, setScanning] = useState(false);
  const polling = useRef<AbortController | null>(null);
  useEffect(() => () => polling.current?.abort(), []);

  const rescan = () => {
    if (polling.current) return; // already waiting for a scan
    const ac = new AbortController();
    polling.current = ac;
    setScanning(true);
    waitForScan(ac.signal)
      .catch(() => {})
      .finally(() => {
        polling.current = null;
        if (ac.signal.aborted) return; // the screen is gone
        setScanning(false);
        refetch();
      });
  };
  return { scanning, rescan };
}

async function waitForScan(signal: AbortSignal) {
  let job = await scanLibrary();
  while (job.status === "running" && !signal.aborted) {
    await sleep(POLL_EVERY, signal);
    job = await getJob(job.id, { signal });
  }
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const t = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => {
      clearTimeout(t);
      resolve();
    });
  });
}
