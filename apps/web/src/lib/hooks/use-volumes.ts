import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as volumesApi from "@/lib/api/volumes";

export function useVolumes(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.volumes.all(projectId, applicationId),
    queryFn: () => volumesApi.listVolumes(projectId, applicationId),
  });
}

export function useCreateVolume(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; mount_path: string }) =>
      volumesApi.createVolume(projectId, applicationId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.volumes.all(projectId, applicationId),
      }),
  });
}

export function useDeleteVolume(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      volumeId,
      deleteData,
    }: {
      volumeId: string;
      deleteData: boolean;
    }) => volumesApi.deleteVolume(projectId, applicationId, volumeId, deleteData),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.volumes.all(projectId, applicationId),
      }),
  });
}
