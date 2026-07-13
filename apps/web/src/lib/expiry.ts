/** Days from now until `iso`, floored. Negative once past. */
export function daysUntil(iso: string | null | undefined): number | null {
  if (!iso) return null;
  return Math.floor(
    (new Date(iso).getTime() - Date.now()) / (1000 * 60 * 60 * 24),
  );
}

/** How close to expiry a certificate has to be before we say so in amber. */
export const EXPIRY_WARNING_DAYS = 14;
