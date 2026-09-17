import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as certificatesApi from "@/lib/api/certificates";
import type { UploadCertificate } from "@/lib/api/certificates";

// The full store: every certificate with every SAN, admin-only at the API. The
// enabled flag exists so a non-admin caller does not fire a request that can
// only 403 — see useUsableCertificates for the member-reachable question.
export function useCertificates(enabled = true) {
  return useQuery({
    queryKey: queryKeys.certificates,
    queryFn: certificatesApi.listCertificates,
    enabled,
  });
}

// Certificates that can serve one hostname. Unlike useCertificates this is
// reachable by any signed-in user, which is the whole point: a member may attach
// a certificate but may not list them, so the add-domain picker was empty for
// them and the form refused to submit.
//
// Keyed by hostname and disabled until there is one, so typing a hostname does
// not fire a request per keystroke before it is a plausible name.
export function useUsableCertificates(hostname: string, enabled = true) {
  const trimmed = hostname.trim();
  return useQuery({
    queryKey: queryKeys.usableCertificates(trimmed),
    queryFn: () => certificatesApi.listUsableCertificates(trimmed),
    enabled: enabled && trimmed.length > 0,
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

// enabled exists because this is an install-wide (member-scoped) fetch, and it
// is now called from the per-application domains table as well as the
// certificates page. There it is only needed to name a custom certificate, so
// an app whose domains all use automatic TLS should not poll for it at all.
export function useDomainTLSStatus(enabled = true) {
  return useQuery({
    queryKey: queryKeys.domainTLSStatus,
    queryFn: certificatesApi.listDomainTLSStatus,
    enabled,
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
