import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as envvarsApi from "@/lib/api/envvars";
import type { EnvVarInput } from "@/lib/types";

export function useEnvVars(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.envvars.all(projectId, applicationId),
    queryFn: () => envvarsApi.getEnvVars(projectId, applicationId),
  });
}

export function useRevealEnvVar(projectId: string, applicationId: string) {
  return useMutation({
    mutationFn: (envVarId: string) =>
      envvarsApi.revealEnvVar(projectId, applicationId, envVarId),
  });
}

export function useUpsertEnvVars(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: EnvVarInput[]) =>
      envvarsApi.upsertEnvVars(projectId, applicationId, vars),
    // Also refresh the application: saving stamps the config-changed marker
    // server-side, and the header badge that reports it reads the application
    // detail. Without this the badge would not appear until the next poll.
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({
          queryKey: queryKeys.envvars.all(projectId, applicationId),
        }),
        qc.invalidateQueries({
          queryKey: queryKeys.applications.detail(projectId, applicationId),
        }),
      ]),
  });
}
