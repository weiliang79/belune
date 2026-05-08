import type { Invitation } from "@/lib/types";
import { api } from "./client";

export function listInvitations() {
  return api.get<Invitation[]>("/users/invitations");
}

export function inviteUser(data: { email: string; role: string }) {
  return api.post<Invitation>("/users/invite", data);
}

export function revokeInvitation(invitationId: string) {
  return api.delete<void>(`/users/invitations/${invitationId}`);
}

export function getInvitationByToken(token: string) {
  return api.get<{ email: string; role: string }>(
    `/auth/invitation?token=${encodeURIComponent(token)}`,
  );
}

export function acceptInvitation(data: {
  token: string;
  password: string;
  username?: string;
  first_name?: string;
  last_name?: string;
}) {
  return api.post<{ token: string; user: import("@/lib/types").User }>(
    "/auth/accept-invitation",
    data,
  );
}
