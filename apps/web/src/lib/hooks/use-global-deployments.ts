import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as globalDeploymentsApi from "@/lib/api/global-deployments";
import type { GlobalDeploymentFilters } from "@/lib/api/global-deployments";

export function useGlobalDeployments(params?: GlobalDeploymentFilters) {
  return useQuery({
    queryKey: queryKeys.globalDeployments(params),
    queryFn: () => globalDeploymentsApi.listGlobalDeployments(params),
  });
}
