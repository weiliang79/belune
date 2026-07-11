import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as certificatesApi from "@/lib/api/certificates";
import type { UploadCertificate } from "@/lib/api/certificates";

export function useCertificates() {
  return useQuery({
    queryKey: queryKeys.certificates,
    queryFn: certificatesApi.listCertificates,
  });
}

export function useUploadCertificate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UploadCertificate) =>
      certificatesApi.uploadCertificate(data),
    onSuccess: () => {
      toast.success("Certificate uploaded");
      qc.invalidateQueries({ queryKey: queryKeys.certificates });
    },
    // Validation failures (mismatched key, not PEM, duplicate name) come back
    // with a specific reason — show it rather than a generic message.
    onError: (err) => toast.error(err.message),
  });
}

export function useDeleteCertificate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => certificatesApi.deleteCertificate(id),
    onSuccess: () => {
      toast.success("Certificate deleted");
      qc.invalidateQueries({ queryKey: queryKeys.certificates });
    },
    // A 409 names the domains still serving the certificate.
    onError: (err) => toast.error(err.message),
  });
}

export function useDomainTLSStatus() {
  return useQuery({
    queryKey: queryKeys.domainTLSStatus,
    queryFn: certificatesApi.listDomainTLSStatus,
    // The sweep runs every minute; a domain mid-issuance should not look stuck
    // just because the page was left open.
    refetchInterval: 30000,
  });
}

export function useRecheckDomainTLS(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      certificatesApi.recheckDomainTLS(projectId, applicationId, domainId),
    onSuccess: () => {
      toast.success("Rechecking certificate…");
      // The probe is asynchronous; give it a moment before re-reading.
      setTimeout(() => {
        qc.invalidateQueries({ queryKey: queryKeys.domainTLSStatus });
        qc.invalidateQueries({
          queryKey: queryKeys.domains.all(projectId, applicationId),
        });
      }, 2000);
    },
    onError: (err) => toast.error(err.message),
  });
}
