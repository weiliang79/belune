import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as featuresApi from "@/lib/api/features";
import { BRAND } from "@/lib/brand";

export function useFeatures() {
  return useQuery({
    queryKey: queryKeys.features,
    queryFn: featuresApi.getFeatures,
    staleTime: 3_000,
  });
}

/**
 * The operator-configured instance name, used as the dashboard brand. Falls back
 * to the static BRAND.name until the (public) features request resolves.
 */
export function useInstanceName(): string {
  const { data } = useFeatures();
  return data?.instance_name?.trim() || BRAND.name;
}
