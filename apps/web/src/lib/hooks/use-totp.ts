import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  disableTotp,
  enrollTotp,
  getTotpStatus,
  regenerateRecoveryCodes,
  resetUserTotp,
  verifyTotpEnrollment,
} from "@/lib/api/totp";
import { queryKeys } from "./query-keys";

export function useTotpStatus() {
  return useQuery({
    queryKey: queryKeys.auth.totp,
    queryFn: getTotpStatus,
  });
}

export function useEnrollTotp() {
  return useMutation({ mutationFn: enrollTotp });
}

export function useVerifyTotpEnrollment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => verifyTotpEnrollment(code),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.auth.totp });
    },
  });
}

export function useDisableTotp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ password, code }: { password: string; code: string }) =>
      disableTotp(password, code),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.auth.totp });
    },
  });
}

export function useRegenerateRecoveryCodes() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (password: string) => regenerateRecoveryCodes(password),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.auth.totp });
    },
  });
}

export function useResetUserTotp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => resetUserTotp(userId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.users.all });
    },
  });
}
