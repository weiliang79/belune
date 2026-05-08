import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  listInvitations,
  inviteUser,
  revokeInvitation,
} from "@/lib/api/invitations";
import { queryKeys } from "./query-keys";

export function useInvitations() {
  return useQuery({
    queryKey: queryKeys.invitations,
    queryFn: listInvitations,
  });
}

export function useInviteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string; role: string }) => inviteUser(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.invitations });
    },
  });
}

export function useRevokeInvitation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (invitationId: string) => revokeInvitation(invitationId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.invitations });
    },
  });
}
