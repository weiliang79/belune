import type { RequestLog } from "@/lib/types";
import { api } from "./client";

export function listRequestLogs(params?: { limit?: number; offset?: number }) {
  const query = new URLSearchParams();
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.offset) query.set("offset", String(params.offset));
  const qs = query.toString();
  return api.get<RequestLog[]>(`/requests${qs ? `?${qs}` : ""}`);
}
