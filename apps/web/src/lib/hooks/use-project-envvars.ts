import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as projectEnvvarsApi from "@/lib/api/project-envvars";

export function useProjectEnvVars(projectId: string) {
  return useQuery({
    queryKey: queryKeys.projectEnvvars.all(projectId),
    queryFn: () => projectEnvvarsApi.getProjectEnvVars(projectId),
  });
}

export function useUpsertProjectEnvVars(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { key: string; value: string; is_secret: boolean }[]) =>
      projectEnvvarsApi.upsertProjectEnvVars(projectId, vars),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.projectEnvvars.all(projectId),
      }),
  });
}
