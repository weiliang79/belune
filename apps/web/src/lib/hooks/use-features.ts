import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as featuresApi from "@/lib/api/features";

export function useFeatures() {
  return useQuery({
    queryKey: queryKeys.features,
    queryFn: featuresApi.getFeatures,
    staleTime: 3_000,
  });
}
