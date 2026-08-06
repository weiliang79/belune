import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as projectEnvvarsApi from "@/lib/api/project-envvars";
import type { EnvVarInput } from "@/lib/types";

export function useProjectEnvVars(projectId: string) {
  return useQuery({
    queryKey: queryKeys.projectEnvvars.all(projectId),
    queryFn: () => projectEnvvarsApi.getProjectEnvVars(projectId),
  });
}

export function useRevealProjectEnvVar(projectId: string) {
  return useMutation({
    mutationFn: (envVarId: string) =>
      projectEnvvarsApi.revealProjectEnvVar(projectId, envVarId),
  });
}

export function useUpsertProjectEnvVars(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: EnvVarInput[]) =>
      projectEnvvarsApi.upsertProjectEnvVars(projectId, vars),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.projectEnvvars.all(projectId),
      }),
  });
}
