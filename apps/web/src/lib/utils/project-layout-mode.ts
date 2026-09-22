/**
 * What $projectId.tsx's ProjectLayout should render, given the router's live
 * (optimistic) pathname and its settled (resolved) pathname:
 *
 * - "chrome": show the project header + tabs + <Outlet/> as normal.
 * - "outlet": hide the project chrome and render a bare <Outlet/> — either
 *   already inside a child-detail page (switching tabs, or jumping straight
 *   to a different app/database), where the nested layout owns its own
 *   loading state, or a direct/cold load at a child-detail URL.
 * - "transitioning": freshly crossing in from project overview/settings/etc
 *   into a child-detail page whose route hasn't mounted yet — render a
 *   plain skeleton instead of <Outlet/>, which would otherwise keep
 *   rendering the project's now-unrelated previous match.
 *
 * Extracted from the component so this predicate — which took two rounds of
 * live-reproduced regressions to get right — is covered by a fast unit test
 * instead of only a browser click-through.
 */
export type ProjectLayoutMode = "chrome" | "outlet" | "transitioning";

export function projectLayoutMode(
  currentPath: string,
  resolvedPath: string,
): ProjectLayoutMode {
  if (!isChildDetailPath(currentPath)) return "chrome";
  return isChildDetailPath(resolvedPath) ? "outlet" : "transitioning";
}

function isChildDetailPath(pathname: string): boolean {
  return (
    pathname.includes("/applications/") || pathname.includes("/databases/")
  );
}
