import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as fileMountsApi from "@/lib/api/file-mounts";

export function useFileMounts(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.fileMounts.all(projectId, applicationId),
    queryFn: () => fileMountsApi.listFileMounts(projectId, applicationId),
  });
}

export function useRevealFileMount(projectId: string, applicationId: string) {
  return useMutation({
    mutationFn: (fileMountId: string) =>
      fileMountsApi.revealFileMount(projectId, applicationId, fileMountId),
  });
}

export function useCreateFileMount(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof fileMountsApi.createFileMount>[2]) =>
      fileMountsApi.createFileMount(projectId, applicationId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.fileMounts.all(projectId, applicationId),
      }),
  });
}

export function useUpdateFileMount(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      fileMountId,
      ...data
    }: {
      fileMountId: string;
    } & Parameters<typeof fileMountsApi.updateFileMount>[3]) =>
      fileMountsApi.updateFileMount(projectId, applicationId, fileMountId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.fileMounts.all(projectId, applicationId),
      }),
  });
}

export function useDeleteFileMount(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (fileMountId: string) =>
      fileMountsApi.deleteFileMount(projectId, applicationId, fileMountId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.fileMounts.all(projectId, applicationId),
      }),
  });
}
