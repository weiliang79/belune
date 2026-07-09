import { useEffect } from "react";
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Accent = "violet" | "emerald";

interface AccentState {
  accent: Accent;
  setAccent: (accent: Accent) => void;
  toggle: () => void;
}

/** Reflect the chosen accent onto <html data-accent> (drives the CSS tokens). */
function applyAccent(accent: Accent) {
  if (typeof document === "undefined") return;
  const el = document.documentElement;
  if (accent === "emerald") {
    el.dataset.accent = "emerald";
  } else {
    delete el.dataset.accent;
  }
}

export const useAccentStore = create<AccentState>()(
  persist(
    (set, get) => ({
      accent: "violet",
      setAccent: (accent) => set({ accent }),
      toggle: () =>
        set({ accent: get().accent === "violet" ? "emerald" : "violet" }),
    }),
    { name: "belune-accent" },
  ),
);

/**
 * Keep <html data-accent> in sync with the persisted accent. Call once near the
 * app root so it applies everywhere (including pre-auth screens).
 */
export function useAccentSync() {
  const accent = useAccentStore((s) => s.accent);
  useEffect(() => {
    applyAccent(accent);
  }, [accent]);
}
