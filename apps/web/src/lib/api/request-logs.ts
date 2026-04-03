import type { RequestLog } from "@/lib/types";
import { api } from "./client";

export interface RequestLogFilters {
  limit?: number;
  offset?: number;
  application_id?: string;
  status_min?: number;
  status_max?: number;
  from?: string;
  to?: string;
}

export function listRequestLogs(params?: RequestLogFilters) {
  const query = new URLSearchParams();
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.offset) query.set("offset", String(params.offset));
  if (params?.application_id) query.set("application_id", params.application_id);
  if (params?.status_min != null) query.set("status_min", String(params.status_min));
  if (params?.status_max != null) query.set("status_max", String(params.status_max));
  if (params?.from) query.set("from", params.from);
  if (params?.to) query.set("to", params.to);
  const qs = query.toString();
  return api.get<RequestLog[]>(`/requests${qs ? `?${qs}` : ""}`);
}
