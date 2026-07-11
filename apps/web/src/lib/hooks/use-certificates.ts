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
