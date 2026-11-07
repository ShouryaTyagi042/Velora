import { useFocusEffect } from "expo-router";
import { useCallback, useRef } from "react";

// A count of changes made from this phone that can take an item out of a list or put one in:
// moving to the bin, restoring, deleting, favouriting, tagging. Lists don't refetch every time
// they're shown (that would throw away the pages already scrolled through); they refetch when
// they come back into view and this has moved since they last loaded.
let version = 0;

export function libraryChanged() {
  version++;
}

// useRefetchOnLibraryChange calls refetch when the screen regains focus after a library change.
export function useRefetchOnLibraryChange(refetch: () => void) {
  const seen = useRef(version);
  useFocusEffect(
    useCallback(() => {
      if (seen.current === version) return;
      seen.current = version;
      refetch();
    }, [refetch]),
  );
}
