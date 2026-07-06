import { formatBytes } from "@/lib/utils/format";

/** Human size, or an em-dash when the daemon didn't compute it (size < 0). */
export function sizeLabel(bytes: number): string {
  if (bytes == null || bytes < 0) return "—";
  return formatBytes(bytes);
}

/** Short Docker id: strips any `sha256:` prefix and truncates to 12 chars. */
export function shortId(id: string): string {
  const bare = id.startsWith("sha256:") ? id.slice("sha256:".length) : id;
  return bare.slice(0, 12);
}
