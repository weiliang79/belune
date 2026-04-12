import { api } from "./client";

export function createTerminalSession(
  projectId: string,
  applicationId: string,
  shell: string,
): Promise<{ session_id: string }> {
  return api.post(
    `/projects/${projectId}/applications/${applicationId}/terminal`,
    { shell },
  );
}
