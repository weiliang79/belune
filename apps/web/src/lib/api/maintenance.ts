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
