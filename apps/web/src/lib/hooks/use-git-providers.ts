import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as gitProvidersApi from "@/lib/api/git-providers";
import type { SaveGitProviderConfig } from "@/lib/api/git-providers";

export function useGitProviderConfigs() {
  return useQuery({
    queryKey: queryKeys.gitProviders,
    queryFn: gitProvidersApi.listGitProviderConfigs,
    refetchInterval: 60000,
  });
}

export function useSaveGitProviderConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SaveGitProviderConfig) =>
      gitProvidersApi.saveGitProviderConfig(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.gitProviders }),
    onError: (err) => toast.error(err.message),
  });
}

export function useDeleteGitProviderConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => gitProvidersApi.deleteGitProviderConfig(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.gitProviders }),
    onError: (err) => toast.error(err.message),
  });
}
