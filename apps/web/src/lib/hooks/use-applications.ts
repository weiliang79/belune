import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as applicationsApi from "@/lib/api/applications";

export function useApplications(projectId: string) {
  return useQuery({
    queryKey: queryKeys.applications.all(projectId),
    queryFn: () => applicationsApi.listApplications(projectId),
    refetchInterval: 5000,
  });
}

export function useApplication(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.applications.detail(projectId, applicationId),
    queryFn: () => applicationsApi.getApplication(projectId, applicationId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      const isTransitional =
        status === "deploying" || status === "building" || status === "pending";
      return isTransitional ? 3000 : false;
    },
  });
}

export function useCreateApplication(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof applicationsApi.createApplication>[1]) =>
      applicationsApi.createApplication(projectId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.applications.all(projectId) });
      qc.invalidateQueries({ queryKey: queryKeys.projects.detail(projectId) });
    },
  });
}

export function useUpdateApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof applicationsApi.updateApplication>[2]) =>
      applicationsApi.updateApplication(projectId, applicationId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.applications.all(projectId) });
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      });
    },
  });
}

export function useDeleteApplication(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (applicationId: string) =>
      applicationsApi.deleteApplication(projectId, applicationId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.applications.all(projectId) });
      qc.invalidateQueries({ queryKey: queryKeys.projects.detail(projectId) });
    },
  });
}

export function useDeployApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.deployApplication(projectId, applicationId),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      });
      qc.invalidateQueries({
        queryKey: queryKeys.deployments.all(projectId, applicationId),
      });
    },
  });
}

export function useStopApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.stopApplication(projectId, applicationId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      }),
  });
}

export function useStartApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.startApplication(projectId, applicationId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      }),
  });
}

export function useRestartApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.restartApplication(projectId, applicationId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      }),
  });
}

export function useUpdateWebhook(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof applicationsApi.updateWebhook>[2]) =>
      applicationsApi.updateWebhook(projectId, applicationId, data),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      });
    },
  });
}
