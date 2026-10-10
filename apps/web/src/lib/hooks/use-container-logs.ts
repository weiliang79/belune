import { useQuery } from "@tanstack/react-query";
import * as logsApi from "@/lib/api/container-logs";
import type {
  ContainerLogParams,
  ContainerLogSource,
} from "@/lib/api/container-logs";
import { queryKeys } from "./query-keys";

export function useContainerLogs(
  source: ContainerLogSource,
  projectId: string,
  sourceId: string,
  params?: ContainerLogParams,
) {
  return useQuery({
    queryKey: queryKeys.containerLogs.history(
      source,
      projectId,
      sourceId,
      params,
    ),
    queryFn: () =>
      logsApi.listContainerLogs(source, projectId, sourceId, params),
    // ⚠️ Never serve log history from cache. main.tsx sets a 60s global
    // staleTime, and inheriting it here lost lines on screen.
    //
    // Found on a real stack: restart an application, watch the logs run from
    // dying to starting, leave the Logs tab and come back — and the starting
    // lines are gone until a full page refresh. Two things combined. The
    // viewer's live buffer is component state, so leaving the route empties it;
    // and this query was still "fresh", so React Query served the fetch taken
    // BEFORE the restart and did not re-read. Neither source then held the new
    // container's lines. A refresh worked because it bypasses the cache.
    //
    // A log pane is append-only and the newest line is the point of it, so
    // "recent enough" is never the right answer — `usePlatformLogs` reaches the
    // same conclusion for the same reason. Cached data stays on screen while
    // the re-read is in flight, so this costs a request per mount, not a blank
    // pane.
    //
    // ⚠️ This also re-enables a refetch on window focus (the global default,
    // which staleness was gating). That is wanted — it reconciles anything the
    // socket missed — and it is only safe because mergeLogEntries dedupes
    // history against the live buffer. Before that fix this same refetch was
    // what showed every line twice.
    staleTime: 0,
  });
}

export function useContainerLogSessions(
  source: ContainerLogSource,
  projectId: string,
  sourceId: string,
) {
  return useQuery({
    queryKey: queryKeys.containerLogs.sessions(source, projectId, sourceId),
    queryFn: () =>
      logsApi.listContainerLogSessions(source, projectId, sourceId),
    // Sessions change as new deploys land; keep them reasonably fresh but don't
    // hammer — a manual refresh or navigation will pick up new ones.
    refetchInterval: 30_000,
  });
}
