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
