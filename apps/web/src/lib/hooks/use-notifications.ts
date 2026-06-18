import { useCallback } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import { useSSEWithReconnect } from "./use-sse";
import * as notificationsApi from "@/lib/api/notifications";
import type { Notification } from "@/lib/types";

/** Full notification list + unread count (loaded when the bell opens). */
export function useNotifications() {
  return useQuery({
    queryKey: queryKeys.notifications.list,
    queryFn: () => notificationsApi.listNotifications({ limit: 30 }),
  });
}

/**
 * Lightweight always-on unread count for the bell badge. The SSE stream keeps
 * it live; a slow refetch interval is a safety net if the stream drops.
 */
export function useUnreadCount() {
  return useQuery({
    queryKey: queryKeys.notifications.unread,
    queryFn: notificationsApi.getUnreadCount,
    refetchInterval: 60_000,
  });
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => notificationsApi.markNotificationRead(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.notifications.list });
      qc.invalidateQueries({ queryKey: queryKeys.notifications.unread });
    },
  });
}

export function useMarkAllNotificationsRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: notificationsApi.markAllNotificationsRead,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.notifications.list });
      qc.invalidateQueries({ queryKey: queryKeys.notifications.unread });
    },
  });
}

/**
 * Subscribes to the live notification SSE stream. The DB is the source of
 * truth, so on each pushed event we reconcile by invalidating the list +
 * unread queries rather than trusting the payload alone; the payload is used
 * only for a transient toast. Mount once (in the topbar).
 */
export function useNotificationStream() {
  const qc = useQueryClient();

  const onMessage = useCallback(
    (raw: string) => {
      qc.invalidateQueries({ queryKey: queryKeys.notifications.list });
      qc.invalidateQueries({ queryKey: queryKeys.notifications.unread });
      try {
        const n = JSON.parse(raw) as Notification;
        toast(n.title, { description: n.body || undefined });
      } catch {
        // Heartbeats / malformed frames: the invalidation above is enough.
      }
    },
    [qc],
  );

  const { connected } = useSSEWithReconnect(
    "/api/notifications/stream",
    onMessage,
  );
  return { connected };
}
