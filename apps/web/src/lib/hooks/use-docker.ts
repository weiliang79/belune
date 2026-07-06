import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as dockerApi from "@/lib/api/docker";

// Read-only Docker inspect queries. The overview + containers views poll on a
// light interval so status changes surface without a manual refresh; images,
// volumes, and networks change rarely and are fetched on demand.

export function useDockerOverview(enabled = true) {
  return useQuery({
    queryKey: queryKeys.docker.overview,
    queryFn: dockerApi.getDockerOverview,
    refetchInterval: 15_000,
    enabled,
  });
}

export function useDockerContainers(enabled = true) {
  return useQuery({
    queryKey: queryKeys.docker.containers,
    queryFn: dockerApi.listDockerContainers,
    refetchInterval: 15_000,
    enabled,
  });
}

export function useDockerImages(enabled = true) {
  return useQuery({
    queryKey: queryKeys.docker.images,
    queryFn: dockerApi.listDockerImages,
    enabled,
  });
}

export function useDockerVolumes(enabled = true) {
  return useQuery({
    queryKey: queryKeys.docker.volumes,
    queryFn: dockerApi.listDockerVolumes,
    enabled,
  });
}

export function useDockerNetworks(enabled = true) {
  return useQuery({
    queryKey: queryKeys.docker.networks,
    queryFn: dockerApi.listDockerNetworks,
    enabled,
  });
}
