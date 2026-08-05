import type { BackupProvider } from "@/lib/types";

// Short display names for compact contexts (e.g. a destination's type
// badge) — distinct from the longer, parenthetical labels used in the
// create/edit provider dropdown (destination-form-dialog.tsx).
export const PROVIDER_LABELS: Record<BackupProvider, string> = {
  s3: "AWS S3",
  r2: "Cloudflare R2",
  b2: "Backblaze B2",
  wasabi: "Wasabi",
  minio: "MinIO",
  other: "Other",
  local: "Local",
};
