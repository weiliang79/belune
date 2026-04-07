import type { AuditLog } from "@/lib/types";
import { api } from "./client";

export function listAuditLogs(params: { limit: number; offset: number }) {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  return api.get<{ items: AuditLog[]; total: number }>(
    `/audit-logs?${query}`,
  );
}
