import { useState } from "react";

import { scanLibrary } from "@/api/media";

// useRescan is pull-to-refresh for a library screen: rescan the server's folder (so newly copied
// files appear), then refetch the screen's data. A failed scan still refetches what's there.
export function useRescan(refetch: () => void) {
  const [scanning, setScanning] = useState(false);
  const rescan = () => {
    setScanning(true);
    scanLibrary()
      .catch(() => {})
      .finally(() => {
        setScanning(false);
        refetch();
      });
  };
  return { scanning, rescan };
}
