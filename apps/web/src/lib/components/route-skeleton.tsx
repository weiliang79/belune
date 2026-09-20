import { Skeleton } from "@/components/ui/skeleton";

// Generic placeholder for `defaultPendingComponent` — shown in the <Outlet />
// slot while a lazy route chunk loads, so the router stops rendering the
// PREVIOUS route's content during that gap (see main.tsx).
export function RouteSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-4 w-72" />
      </div>
      <Skeleton className="h-32 w-full rounded-lg" />
      <Skeleton className="h-32 w-full rounded-lg" />
    </div>
  );
}
