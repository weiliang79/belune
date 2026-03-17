import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as deploymentsApi from "@/lib/api/deployments";

export function useDeployments(projectId: string, serviceId: string) {
  return useQuery({
    queryKey: queryKeys.deployments.all(projectId, serviceId),
    queryFn: () => deploymentsApi.listDeployments(projectId, serviceId),
  });
}

export function useDeployment(
  projectId: string,
  serviceId: string,
  deploymentId: string,
) {
  return useQuery({
    queryKey: queryKeys.deployments.detail(projectId, serviceId, deploymentId),
    queryFn: () =>
      deploymentsApi.getDeployment(projectId, serviceId, deploymentId),
  });
}
