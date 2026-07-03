import type { ReactNode } from "react";

/**
 * Standard page header: an icon tile beside the title/description, with an
 * optional right-aligned actions slot. Mirrors the detail-page header chrome
 * so every top-level page reads the same.
 */
export function PageHeader({
  icon,
  title,
  description,
  actions,
}: {
  icon: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <div className="bg-elev text-text-muted grid size-11 shrink-0 place-items-center rounded-xl">
          {icon}
        </div>
        <div className="min-w-0">
          <h1 className="truncate text-2xl font-semibold tracking-tight">
            {title}
          </h1>
          {description != null && (
            <p className="text-muted-foreground text-sm">{description}</p>
          )}
        </div>
      </div>
      {actions}
    </div>
  );
}
