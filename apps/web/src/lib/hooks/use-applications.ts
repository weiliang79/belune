import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as applicationsApi from "@/lib/api/applications";

export function useApplications(projectId: string) {
  return useQuery({
    queryKey: queryKeys.applications.all(projectId),
    queryFn: () => applicationsApi.listApplications(projectId),
    refetchInterval: (query) => {
      const hasTransitional = query.state.data?.some((app) =>
        ["building", "deploying", "pending"].includes(app.status),
      );
      return hasTransitional ? 3000 : 10000;
    },
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
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
  });
}

export function useBuildCache(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.applications.buildCache(projectId, applicationId),
    queryFn: () => applicationsApi.getBuildCache(projectId, applicationId),
  });
}

export function useClearBuildCache(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.clearBuildCache(projectId, applicationId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.applications.buildCache(projectId, applicationId),
      }),
    onError: (error) => toast.error(error.message),
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
    onError: (error) => toast.error(error.message),
  });
}
