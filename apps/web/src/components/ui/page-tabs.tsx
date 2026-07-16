import { Link } from "@tanstack/react-router";
import type { ComponentType, ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * Unified page-level tab bar (underline style) used across detail/settings
 * pages. Two modes share one look:
 *
 *   - <PageTabLinks> — Link mode. Each tab is a real <Link> with its own URL,
 *     so middle-click / cmd-click opens the tab in a new browser tab. Use for
 *     tabs that map to nested routes (app detail, project detail).
 *   - <PageTabs> — controlled mode. Value + onValueChange, for pages that keep
 *     their tab in a `?tab=` search param (server, docker, git, DB detail).
 *
 * For section *filters* (not navigation) keep SegmentedControl instead.
 */

type IconType = ComponentType<{ className?: string; "aria-hidden"?: boolean }>;

const tabRow = "flex gap-1 overflow-x-auto border-b";
const tabBase =
  "group flex items-center gap-1.5 border-b-2 px-4 py-2 text-sm font-medium whitespace-nowrap transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-t-sm";
const tabActive = "border-primary text-foreground";
const tabInactive =
  "border-transparent text-muted-foreground hover:text-foreground";
// Accent-color the icon on the active tab. Link mode surfaces active state via
// TanStack's data-status="active" on the anchor; controlled mode sets
// data-active on the button. The tab is a `group`, so the icon keys off either.
const tabIcon =
  "size-4 shrink-0 transition-colors group-data-[status=active]:text-primary group-data-[active]:text-primary";

/** Pulsing dot for "something is happening on this tab" (e.g. a live deploy). */
function LiveDot({ label }: { label: string }) {
  return (
    <span aria-label={label} className="relative flex size-2">
      <span className="bg-status-ready absolute inline-flex size-full animate-ping rounded-full opacity-75" />
      <span className="bg-status-ready relative inline-flex size-2 rounded-full" />
    </span>
  );
}

function TabInner({
  Icon,
  label,
  live,
  liveLabel,
}: {
  Icon?: IconType;
  label: ReactNode;
  live?: boolean;
  liveLabel?: string;
}) {
  return (
    <>
      {Icon && <Icon aria-hidden={true} className={tabIcon} />}
      {label}
      {live && <LiveDot label={liveLabel ?? "In progress"} />}
    </>
  );
}

// ── Link mode ──────────────────────────────────────────────────────────────

export interface PageTabLink {
  to: string;
  label: string;
  icon?: IconType;
  /** Match the path exactly (for an index/overview tab). */
  exact?: boolean;
  live?: boolean;
  liveLabel?: string;
}

export function PageTabLinks({
  items,
  ariaLabel,
  className,
}: {
  items: PageTabLink[];
  ariaLabel: string;
  className?: string;
}) {
  return (
    <nav aria-label={ariaLabel} className={cn(tabRow, className)}>
      {items.map((tab) => (
        <Link
          key={tab.to}
          to={tab.to as never}
          activeOptions={tab.exact ? { exact: true } : undefined}
          className={tabBase}
          activeProps={{ className: tabActive }}
          inactiveProps={{ className: tabInactive }}
        >
          <TabInner
            Icon={tab.icon}
            label={tab.label}
            live={tab.live}
            liveLabel={tab.liveLabel}
          />
        </Link>
      ))}
    </nav>
  );
}

// ── Controlled mode ─────────────────────────────────────────────────────────

export interface PageTab<V extends string> {
  value: V;
  label: string;
  icon?: IconType;
  live?: boolean;
  liveLabel?: string;
}

export function PageTabs<V extends string>({
  items,
  value,
  onValueChange,
  ariaLabel,
  className,
}: {
  items: PageTab<V>[];
  value: V;
  onValueChange: (value: V) => void;
  ariaLabel: string;
  className?: string;
}) {
  return (
    <nav
      role="tablist"
      aria-label={ariaLabel}
      className={cn(tabRow, className)}
    >
      {items.map((tab) => {
        const active = tab.value === value;
        return (
          <button
            key={tab.value}
            type="button"
            role="tab"
            aria-selected={active}
            data-active={active ? "" : undefined}
            onClick={() => onValueChange(tab.value)}
            className={cn(tabBase, active ? tabActive : tabInactive)}
          >
            <TabInner
              Icon={tab.icon}
              label={tab.label}
              live={tab.live}
              liveLabel={tab.liveLabel}
            />
          </button>
        );
      })}
    </nav>
  );
}
