import type { Notification } from "@/lib/types";
import { api } from "./client";

export interface NotificationListResponse {
  items: Notification[];
  unread: number;
}

export function listNotifications(params?: {
  limit?: number;
  offset?: number;
}) {
  const query = new URLSearchParams();
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.offset) query.set("offset", String(params.offset));
  const qs = query.toString();
  return api.get<NotificationListResponse>(
    `/notifications${qs ? `?${qs}` : ""}`,
  );
}

export function getUnreadCount() {
  return api.get<{ unread: number }>("/notifications/unread-count");
}

export function markNotificationRead(id: string) {
  return api.post<Notification>(`/notifications/${id}/read`);
}

export function markAllNotificationsRead() {
  return api.post<void>("/notifications/read-all");
}
