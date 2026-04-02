import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as globalDeploymentsApi from "@/lib/api/global-deployments";

export function useGlobalDeployments(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: queryKeys.globalDeployments(params?.limit, params?.offset),
    queryFn: () => globalDeploymentsApi.listGlobalDeployments(params),
  });
}
