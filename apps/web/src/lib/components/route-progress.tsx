import { useEffect } from "react";
import { useRouterState } from "@tanstack/react-router";
import { useProgress } from "@bprogress/react";

// Slim top progress bar for route navigation. Driven ONLY by TanStack Router's
// navigation state — it covers the gap between a nav click and the route
// component mounting (notably lazy code-split chunk loads), which in-component
// skeletons can't show because the component isn't mounted yet.
//
// Deliberately NOT wired to React Query's fetching state: the app polls in the
// background (status-based refetch), so a fetch-driven bar would animate almost
// constantly. Per-component skeletons handle the post-mount data-loading gap.
//
// Must render inside <ProgressProvider> (see __root.tsx) so useProgress() has a
// context. The 150ms start delay keeps instant, cached navigations from
// flashing the bar — only navigations slower than that show it.
export function RouteProgress() {
  const { start, stop } = useProgress();
  const isNavigating = useRouterState({
    select: (s) => s.status === "pending",
  });

  useEffect(() => {
    if (!isNavigating) return;
    // Delay the bar so instant/cached navigations don't flash it. We manage the
    // delay with our own timer (rather than start()'s built-in delay arg) so the
    // cleanup can cancel a still-pending start. Otherwise a navigation that
    // finishes inside the delay window — common on a fresh page load / refresh,
    // where the initial route often resolves in under 150ms — fires the delayed
    // start AFTER we've already called stop(), leaving the bar stuck at the top.
    const timer = setTimeout(() => start(), 150);
    return () => {
      clearTimeout(timer);
      stop();
    };
  }, [isNavigating, start, stop]);

  return null;
}
