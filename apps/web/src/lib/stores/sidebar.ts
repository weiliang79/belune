import { create } from "zustand";
import { persist } from "zustand/middleware";

interface SidebarState {
  isOpen: boolean;
  toggle: () => void;
  setOpen: (open: boolean) => void;
}

export const useSidebarStore = create<SidebarState>()(
  persist(
    (set) => ({
      isOpen: true,
      toggle: () => set((s) => ({ isOpen: !s.isOpen })),
      setOpen: (isOpen) => set({ isOpen }),
    }),
    { name: "sidebar" },
  ),
);
