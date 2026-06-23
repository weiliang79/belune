import { useEffect } from "react";
import { useBreadcrumbStore } from "@/lib/stores/breadcrumb";

/**
 * Register a dynamic breadcrumb label (e.g. a project or application name) so
 * the Topbar can resolve it for the current route. No-op until both id and
 * label are available.
 */
export function useBreadcrumbLabel(
  id: string | undefined,
  label: string | undefined,
) {
  const setLabel = useBreadcrumbStore((s) => s.setLabel);
  useEffect(() => {
    if (id && label) setLabel(id, label);
  }, [id, label, setLabel]);
}
