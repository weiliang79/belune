import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import { listProjectBackups } from "@/lib/api/project-backups";

export function useProjectBackups(projectId: string) {
  return useQuery({
    queryKey: queryKeys.projectBackups(projectId),
    queryFn: () => listProjectBackups(projectId),
    // Poll faster while any backup is running so rows resolve promptly.
    refetchInterval: (query) =>
      query.state.data?.some((b) => b.status === "running") ? 3000 : false,
  });
}
