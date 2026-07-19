import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as smtpApi from "@/lib/api/smtp-settings";
import type { SaveSmtpSettings } from "@/lib/api/smtp-settings";

export function useSmtpSettings() {
  return useQuery({
    queryKey: queryKeys.smtpSettings,
    queryFn: () => smtpApi.getSmtpSettings(),
  });
}

export function useUpdateSmtpSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SaveSmtpSettings) => smtpApi.updateSmtpSettings(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.smtpSettings }),
  });
}

export function useTestSmtpSettings() {
  return useMutation({
    mutationFn: (data: SaveSmtpSettings & { to: string }) =>
      smtpApi.testSmtpSettings(data),
  });
}
