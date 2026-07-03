import type { FileMount } from "@/lib/types";
import { api } from "./client";

export function listFileMounts(projectId: string, applicationId: string) {
  return api.get<FileMount[]>(
    `/projects/${projectId}/applications/${applicationId}/file-mounts`,
  );
}

export function revealFileMount(
  projectId: string,
  applicationId: string,
  fileMountId: string,
) {
  return api.get<{ content: string }>(
    `/projects/${projectId}/applications/${applicationId}/file-mounts/${fileMountId}/reveal`,
  );
}

export function createFileMount(
  projectId: string,
  applicationId: string,
  data: {
    mount_path: string;
    content: string;
    is_secret: boolean;
    file_mode?: string;
  },
) {
  return api.post<FileMount>(
    `/projects/${projectId}/applications/${applicationId}/file-mounts`,
    data,
  );
}

export function updateFileMount(
  projectId: string,
  applicationId: string,
  fileMountId: string,
  data: {
    // Omit content to keep the stored value (e.g. editing a secret's metadata).
    content?: string;
    is_secret: boolean;
    file_mode?: string;
  },
) {
  return api.put<FileMount>(
    `/projects/${projectId}/applications/${applicationId}/file-mounts/${fileMountId}`,
    data,
  );
}

export function deleteFileMount(
  projectId: string,
  applicationId: string,
  fileMountId: string,
) {
  return api.delete<{ status: string }>(
    `/projects/${projectId}/applications/${applicationId}/file-mounts/${fileMountId}`,
  );
}
