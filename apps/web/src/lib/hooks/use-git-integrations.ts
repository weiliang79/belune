import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as api from "@/lib/api/git-integrations";
import type { GitProvider } from "@/lib/api/git-providers";

export function useGitIntegrations() {
  return useQuery({
    queryKey: queryKeys.gitIntegrations,
    queryFn: api.listGitIntegrations,
  });
}

export function useAvailableProviders() {
  return useQuery({
    queryKey: queryKeys.gitAvailableProviders,
    queryFn: api.listAvailableProviders,
  });
}

export function useStartGitConnect() {
  return useMutation({
    mutationFn: ({
      provider,
      baseUrl,
    }: {
      provider: GitProvider;
      baseUrl?: string;
    }) => api.startGitConnect(provider, baseUrl),
    onSuccess: ({ auth_url }) => {
      window.location.href = auth_url;
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useIntegrationRepos(integrationId: string | undefined) {
  return useQuery({
    queryKey: ["git-integration-repos", integrationId],
    queryFn: () => api.listIntegrationRepos(integrationId!),
    enabled: !!integrationId,
  });
}

export function useIntegrationBranches(
  integrationId: string | undefined,
  repo: string | undefined,
) {
  return useQuery({
    queryKey: ["git-integration-branches", integrationId, repo],
    queryFn: () => api.listIntegrationBranches(integrationId!, repo!),
    enabled: !!integrationId && !!repo,
  });
}

export function useDeleteGitIntegration() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteGitIntegration(id),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.gitIntegrations }),
    onError: (err) => toast.error(err.message),
  });
}
