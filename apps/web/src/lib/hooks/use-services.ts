import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as servicesApi from "@/lib/api/services";

export function useServices(projectId: string) {
  return useQuery({
    queryKey: queryKeys.services.all(projectId),
    queryFn: () => servicesApi.listServices(projectId),
  });
}

export function useService(projectId: string, serviceId: string) {
  return useQuery({
    queryKey: queryKeys.services.detail(projectId, serviceId),
    queryFn: () => servicesApi.getService(projectId, serviceId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      const isTransitional =
        status === "deploying" || status === "building" || status === "pending";
      return isTransitional ? 3000 : false;
    },
  });
}

export function useCreateService(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof servicesApi.createService>[1]) =>
      servicesApi.createService(projectId, data),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.services.all(projectId) }),
  });
}

export function useUpdateService(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof servicesApi.updateService>[2]) =>
      servicesApi.updateService(projectId, serviceId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.services.all(projectId) });
      qc.invalidateQueries({
        queryKey: queryKeys.services.detail(projectId, serviceId),
      });
    },
  });
}

export function useDeleteService(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (serviceId: string) =>
      servicesApi.deleteService(projectId, serviceId),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: queryKeys.services.all(projectId) }),
  });
}

export function useDeployService(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => servicesApi.deployService(projectId, serviceId),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.services.detail(projectId, serviceId),
      });
      qc.invalidateQueries({
        queryKey: queryKeys.deployments.all(projectId, serviceId),
      });
    },
  });
}

export function useStopService(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => servicesApi.stopService(projectId, serviceId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.services.detail(projectId, serviceId),
      }),
  });
}

export function useStartService(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => servicesApi.startService(projectId, serviceId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.services.detail(projectId, serviceId),
      }),
  });
}

export function useRestartService(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => servicesApi.restartService(projectId, serviceId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.services.detail(projectId, serviceId),
      }),
  });
}

export function useUpdateWebhook(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof servicesApi.updateWebhook>[2]) =>
      servicesApi.updateWebhook(projectId, serviceId, data),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.services.detail(projectId, serviceId),
      });
    },
  });
}
