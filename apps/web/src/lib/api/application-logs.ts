import type { ApplicationLog } from "@/lib/types";
import { api } from "./client";

export interface ApplicationLogParams {
  limit?: number;
  offset?: number;
  q?: string;
  stream?: "stdout" | "stderr" | "";
  since?: string;
  until?: string;
}

export function listApplicationLogs(
  projectId: string,
  applicationId: string,
  params?: ApplicationLogParams,
) {
  const query = new URLSearchParams();
  if (params?.limit) query.set("limit", String(params.limit));
  if (params?.offset) query.set("offset", String(params.offset));
  if (params?.q) query.set("q", params.q);
  if (params?.stream) query.set("stream", params.stream);
  if (params?.since) query.set("since", params.since);
  if (params?.until) query.set("until", params.until);
  const qs = query.toString();
  return api.get<ApplicationLog[]>(
    `/projects/${projectId}/applications/${applicationId}/logs/history${qs ? `?${qs}` : ""}`,
  );
}
