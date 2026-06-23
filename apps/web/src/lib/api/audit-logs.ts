import type { AuditLog } from "@/lib/types";
import { api } from "./client";

export interface AuditLogFilters {
  limit: number;
  offset: number;
  action?: string;
  user_id?: string;
  resource_type?: string;
  resource_id?: string;
  from?: string;
  to?: string;
}

function toQuery(params: AuditLogFilters): string {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  if (params.action) query.set("action", params.action);
  if (params.user_id) query.set("user_id", params.user_id);
  if (params.resource_type) query.set("resource_type", params.resource_type);
  if (params.resource_id) query.set("resource_id", params.resource_id);
  if (params.from) query.set("from", params.from);
  if (params.to) query.set("to", params.to);
  return query.toString();
}

export function listAuditLogs(params: AuditLogFilters) {
  return api.get<{ items: AuditLog[]; total: number }>(
    `/audit-logs?${toQuery(params)}`,
  );
}

export function listAuditActions() {
  return api.get<string[]>("/audit-logs/actions");
}

/** Cookie-authenticated GET — open directly so the browser downloads the file. */
export function auditExportUrl(
  params: Omit<AuditLogFilters, "limit" | "offset">,
) {
  const query = new URLSearchParams();
  if (params.action) query.set("action", params.action);
  if (params.user_id) query.set("user_id", params.user_id);
  if (params.resource_type) query.set("resource_type", params.resource_type);
  if (params.resource_id) query.set("resource_id", params.resource_id);
  if (params.from) query.set("from", params.from);
  if (params.to) query.set("to", params.to);
  const qs = query.toString();
  return `/api/audit-logs/export${qs ? `?${qs}` : ""}`;
}
