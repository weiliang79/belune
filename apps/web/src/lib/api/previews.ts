import { api } from "./client";

export interface PreviewConfig {
  preview_branch_pattern: string;
  preview_domain_template: string;
}

export interface PreviewApp {
  id: string;
  name: string;
  slug: string;
  branch: string;
  status: string;
  last_activity_at: string;
  hostname?: string;
}

export function updatePreviewConfig(
  projectId: string,
  applicationId: string,
  data: Partial<PreviewConfig>,
) {
  return api.put<PreviewConfig>(
    `/projects/${projectId}/applications/${applicationId}/previews/config`,
    data,
  );
}

export function listPreviews(projectId: string, applicationId: string) {
  return api.get<{ previews: PreviewApp[] }>(
    `/projects/${projectId}/applications/${applicationId}/previews`,
  );
}

export function deletePreview(
  projectId: string,
  applicationId: string,
  previewId: string,
) {
  return api.delete<{ status: string }>(
    `/projects/${projectId}/applications/${applicationId}/previews/${previewId}`,
  );
}
