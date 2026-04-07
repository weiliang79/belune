import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as gitCredsApi from "@/lib/api/git-credentials";

export function useGitCredentials() {
  return useQuery({
    queryKey: queryKeys.gitCredentials,
    queryFn: gitCredsApi.listGitCredentials,
  });
}

export function useCreateGitCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof gitCredsApi.createGitCredential>[0]) =>
      gitCredsApi.createGitCredential(data),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.gitCredentials }),
  });
}

export function useUpdateGitCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...data
    }: { id: string } & Parameters<typeof gitCredsApi.updateGitCredential>[1]) =>
      gitCredsApi.updateGitCredential(id, data),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.gitCredentials }),
  });
}

export function useDeleteGitCredential() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => gitCredsApi.deleteGitCredential(id),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.gitCredentials }),
  });
}
