import type { ErrorComponentProps } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { isNotFoundError } from "@/lib/utils/query-error";

/**
 * The error UI for a route whose data could not be read.
 *
 * Distinguishes the two cases a reader actually cares about, because the app
 * used to show the wrong one: a 404 means the resource is genuinely gone
 * (someone deleted it, or the link is stale) and retrying will not help,
 * while anything else — a restarting API, a dropped connection, a 500 — is a
 * failure to *reach* the answer, where retrying is the whole remedy.
 * Collapsing the second into the first is what made a brief API restart look
 * like "your project no longer exists".
 */
export function RouteError({ error, reset }: ErrorComponentProps) {
  const notFound = isNotFoundError(error);

  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <p className="text-destructive mb-2 font-medium">
        {notFound ? "Not found" : "Something went wrong"}
      </p>
      <p className="text-muted-foreground mb-4 max-w-sm text-sm">
        {notFound
          ? "This item no longer exists, or you do not have access to it."
          : error instanceof Error
            ? error.message
            : "An unexpected error occurred."}
      </p>
      {!notFound && (
        <Button variant="outline" size="sm" onClick={reset}>
          Try again
        </Button>
      )}
    </div>
  );
}
