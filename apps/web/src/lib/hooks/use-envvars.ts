import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as envvarsApi from "@/lib/api/envvars";

export function useEnvVars(projectId: string, serviceId: string) {
  return useQuery({
    queryKey: queryKeys.envvars.all(projectId, serviceId),
    queryFn: () => envvarsApi.getEnvVars(projectId, serviceId),
  });
}

export function useUpsertEnvVars(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { key: string; value: string; is_secret: boolean }[]) =>
      envvarsApi.upsertEnvVars(projectId, serviceId, vars),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.envvars.all(projectId, serviceId),
      }),
  });
}
