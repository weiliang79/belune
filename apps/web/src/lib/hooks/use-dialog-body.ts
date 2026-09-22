import { useState } from "react";

/**
 * Remount a dialog's body on every open, and keep it mounted — with the
 * props it opened with — while the dialog animates closed.
 *
 * Gating the body on `open` directly (`{open && <Body/>}`) resets it for
 * free, but also unmounts it the instant `open` goes false, so the dialog
 * animates closed as an empty box instead of fading out its content. And a
 * parent that clears its edit target on close would swap an edit form for a
 * blank "new" one mid-animation. So: the key bumps only on the false→true
 * transition, and the target is frozen at what it was while open.
 *
 * React's adjust-state-while-rendering pattern (compare against last
 * render's `open` in state) — no effect, no flash of the previous content.
 */
export function useDialogBody<T>(
  open: boolean,
  target: T,
): { key: number; target: T } {
  const [held, setHeld] = useState({ open, target, key: 0 });
  if (open !== held.open || (open && target !== held.target)) {
    setHeld({
      open,
      target: open ? target : held.target,
      key: open && !held.open ? held.key + 1 : held.key,
    });
  }
  return { key: held.key, target: held.target };
}
