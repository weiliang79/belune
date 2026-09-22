// TanStack Form's field.state.meta.errors mixes plain strings (from custom
// function validators) and `{ message }` objects (from Zod schema
// validators), so callers can't pass it straight to shadcn's FieldError —
// this normalizes the first error to a plain string either way.
export function fieldError(errors: unknown[]): string | undefined {
  const first = errors[0];
  if (!first) return undefined;
  return typeof first === "string"
    ? first
    : (first as { message?: string }).message;
}
