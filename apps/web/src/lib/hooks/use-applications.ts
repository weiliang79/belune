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

export function useUpdateApplicationRuntime(
  projectId: string,
  applicationId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof applicationsApi.updateApplicationRuntime>[2]) =>
      applicationsApi.updateApplicationRuntime(projectId, applicationId, data),
    onSuccess: () => {
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
    // Await BOTH the detail and list refetch so the mutation stays `isPending`
    // until the row's status actually reflects the change. Without awaiting the
    // list query, isPending flips false while the list still shows the old
    // status, briefly rendering the wrong action buttons (e.g. Stop/Restart
    // flashing between the spinner and Start when stopping).
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({
          queryKey: queryKeys.applications.detail(projectId, applicationId),
        }),
        qc.invalidateQueries({
          queryKey: queryKeys.applications.all(projectId),
        }),
      ]),
    onError: (error) => toast.error(error.message),
  });
}

export function useStartApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.startApplication(projectId, applicationId),
    // Await BOTH the detail and list refetch so the mutation stays `isPending`
    // until the row's status actually reflects the change. Without awaiting the
    // list query, isPending flips false while the list still shows the old
    // status, briefly rendering the wrong action buttons (e.g. Stop/Restart
    // flashing between the spinner and Start when stopping).
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({
          queryKey: queryKeys.applications.detail(projectId, applicationId),
        }),
        qc.invalidateQueries({
          queryKey: queryKeys.applications.all(projectId),
        }),
      ]),
    onError: (error) => toast.error(error.message),
  });
}

export function useRestartApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.restartApplication(projectId, applicationId),
    // Await BOTH the detail and list refetch so the mutation stays `isPending`
    // until the row's status actually reflects the change. Without awaiting the
    // list query, isPending flips false while the list still shows the old
    // status, briefly rendering the wrong action buttons (e.g. Stop/Restart
    // flashing between the spinner and Start when stopping).
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({
          queryKey: queryKeys.applications.detail(projectId, applicationId),
        }),
        qc.invalidateQueries({
          queryKey: queryKeys.applications.all(projectId),
        }),
      ]),
    onError: (error) => toast.error(error.message),
  });
}

// Reload and Rebuild both create a new deployment, so they invalidate the
// deployments list as well as the application detail (mirrors useDeployApplication).
export function useReloadApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.reloadApplication(projectId, applicationId),
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

export function useRebuildApplication(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.rebuildApplication(projectId, applicationId),
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

// Reports whether the deploy hook is enabled. Never carries the token itself —
// use useRevealDeployHook for that.
export function useDeployHook(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.applications.deployHook(projectId, applicationId),
    queryFn: () => applicationsApi.getDeployHook(projectId, applicationId),
  });
}

export function useGenerateDeployHook(
  projectId: string,
  applicationId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      applicationsApi.generateDeployHook(projectId, applicationId),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.applications.deployHook(projectId, applicationId),
      });
    },
    onError: (error) => toast.error(error.message),
  });
}

// A mutation rather than a query so the token is fetched only on an explicit
// click: each call writes an audit-log entry server-side.
export function useRevealDeployHook(projectId: string, applicationId: string) {
  return useMutation({
    mutationFn: () => applicationsApi.revealDeployHook(projectId, applicationId),
    onError: (error) => toast.error(error.message),
  });
}

export function useDeleteDeployHook(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => applicationsApi.deleteDeployHook(projectId, applicationId),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.applications.deployHook(projectId, applicationId),
      });
    },
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
