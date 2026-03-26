import { useEffect, useState } from "react";

export type NodeCollectionViewMode = "grid" | "table";

export function usePersistedViewMode(
  storageKey: string,
  initialMode: NodeCollectionViewMode = "grid",
) {
  const [mode, setMode] = useState<NodeCollectionViewMode>(() => {
    if (typeof window === "undefined") {
      return initialMode;
    }
    const saved = window.localStorage.getItem(storageKey);
    return saved === "grid" || saved === "table" ? saved : initialMode;
  });

  useEffect(() => {
    window.localStorage.setItem(storageKey, mode);
  }, [storageKey, mode]);

  return [mode, setMode] as const;
}
