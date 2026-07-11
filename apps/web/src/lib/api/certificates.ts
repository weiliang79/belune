import { api } from "./client";

export interface Certificate {
  id: string;
  name: string;
  issuer: string;
  subjects: string[];
  not_before: string | null;
  not_after: string | null;
  domain_count: number;
  created_at: string;
}

export interface UploadCertificate {
  name: string;
  cert_pem: string;
  key_pem: string;
}

export function listCertificates() {
  return api.get<Certificate[]>("/certificates");
}

export function uploadCertificate(data: UploadCertificate) {
  return api.post<Certificate>("/certificates", data);
}

export function deleteCertificate(id: string) {
  return api.delete<void>(`/certificates/${id}`);
}

/** One row of the central Domain TLS view. */
export interface DomainTLSStatus {
  id: string;
  hostname: string;
  ssl_mode: string;
  tls_status: string;
  tls_issuer?: string;
  tls_not_after: string | null;
  tls_last_checked_at: string | null;
  tls_error?: string;
  certificate_name?: string;
  application_id: string;
  application_name: string;
  project_id: string;
}

export function listDomainTLSStatus() {
  return api.get<DomainTLSStatus[]>("/domains/tls");
}

export function recheckDomainTLS(
  projectId: string,
  applicationId: string,
  domainId: string,
) {
  return api.post<void>(
    `/projects/${projectId}/applications/${applicationId}/domains/${domainId}/tls/recheck`,
    {},
  );
}
