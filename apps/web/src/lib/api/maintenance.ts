import type { QueueStatus, ReconcilerStatus } from "@/lib/types";
import { api } from "./client";

export type CleanupAction =
  | "deployments"
  | "images"
  | "volumes"
  | "containers"
  | "build_cache";

// Empty/omitted actions = full cleanup (all steps).
export function runCleanup(actions?: CleanupAction[]) {
  return api.post<{ status: string }>(
    "/cleanup",
    actions && actions.length ? { actions } : {},
  );
}

export function getReconcilerStatus() {
  return api.get<ReconcilerStatus>("/proxy/reconciler");
}

export function reconcileProxy() {
  return api.post<ReconcilerStatus>("/proxy/reconcile");
}

export function getQueueStatus() {
  return api.get<QueueStatus>("/maintenance/queue");
}

export function clearQueue() {
  return api.post<{ cleared: number }>("/maintenance/queue/clear");
}

export function clearPendingQueue() {
  return api.post<{ cleared: number }>("/maintenance/queue/clear-pending");
}

export type PlatformService =
  | "belune"
  | "caddy"
  | "redis"
  | "postgres"
  | "buildkit";

export function getPlatformLogs(service: PlatformService) {
  return api.get<{ service: string; content: string }>(
    `/maintenance/logs?service=${encodeURIComponent(service)}`,
  );
}

export type ServerIPSource = "manual" | "env" | "detected" | "unknown";

export function getServerIP() {
  return api.get<{ effective: string; source: ServerIPSource }>(
    "/maintenance/server-ip",
  );
}

export function createHostShellSession(password: string) {
  return api.post<{ session_id: string }>("/maintenance/host-shell", {
    password,
  });
}

export type RestartableService = "caddy" | "redis";

export function restartService(service: RestartableService) {
  return api.post<{ status: string; service: string }>(
    `/maintenance/restart?service=${encodeURIComponent(service)}`,
  );
}
