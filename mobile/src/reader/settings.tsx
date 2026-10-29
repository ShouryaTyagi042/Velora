import { createContext, use, useState, type ReactNode } from "react";

export type ReadingDirection = "horizontal" | "vertical";

type ReaderSettings = {
  direction: ReadingDirection;
  setDirection: (d: ReadingDirection) => void;
};

const Context = createContext<ReaderSettings | null>(null);

// ReaderSettingsProvider holds the reader's settings for the whole app, so the choice survives
// leaving one comic and opening the next. In memory only for now; phase 10 persists it.
export function ReaderSettingsProvider({ children }: { children: ReactNode }) {
  const [direction, setDirection] = useState<ReadingDirection>("horizontal");
  return <Context value={{ direction, setDirection }}>{children}</Context>;
}

export function useReaderSettings(): ReaderSettings {
  const s = use(Context);
  if (!s) throw new Error("useReaderSettings needs a ReaderSettingsProvider above it");
  return s;
}
