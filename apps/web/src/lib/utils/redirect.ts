/**
 * Guard for the `?redirect=` login param. Accepts only absolute in-app paths
 * ("/foo?tab=bar"), rejecting protocol-relative ("//evil.com") and absolute
 * URLs ("https://…") so a crafted login link can't bounce users off-origin.
 */
export function safeRedirectPath(value: unknown): string | undefined {
  if (typeof value !== "string" || value === "") return undefined;
  if (!value.startsWith("/") || value.startsWith("//")) return undefined;
  return value;
}
