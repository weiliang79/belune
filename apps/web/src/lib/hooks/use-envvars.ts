import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as envvarsApi from "@/lib/api/envvars";

export function useEnvVars(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.envvars.all(projectId, applicationId),
    queryFn: () => envvarsApi.getEnvVars(projectId, applicationId),
  });
}

export function useUpsertEnvVars(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { key: string; value: string; is_secret: boolean }[]) =>
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
