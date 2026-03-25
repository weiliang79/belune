import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as databasesApi from "@/lib/api/databases";

export function useDatabases(projectId: string) {
  return useQuery({
    queryKey: queryKeys.databases.all(projectId),
    queryFn: () => databasesApi.listDatabases(projectId),
    refetchInterval: 5000,
  });
}

export function useDatabase(projectId: string, databaseId: string) {
  return useQuery({
    queryKey: queryKeys.databases.detail(projectId, databaseId),
    queryFn: () => databasesApi.getDatabase(projectId, databaseId),
    refetchInterval: (query) =>
      query.state.data?.status === "creating" ? 3000 : false,
  });
}

export function useCreateDatabase(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof databasesApi.createDatabase>[1]) =>
      databasesApi.createDatabase(projectId, data),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.databases.all(projectId) }),
  });
}

export function useDeleteDatabase(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (databaseId: string) =>
      databasesApi.deleteDatabase(projectId, databaseId),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.databases.all(projectId) }),
  });
}
