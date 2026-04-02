import type { ErrorComponentProps } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";

export function RouteError({ error, reset }: ErrorComponentProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <p className="text-destructive mb-2 font-medium">Something went wrong</p>
      <p className="text-muted-foreground mb-4 max-w-sm text-sm">
        {error instanceof Error ? error.message : "An unexpected error occurred."}
      </p>
      <Button variant="outline" size="sm" onClick={reset}>
        Try again
      </Button>
    </div>
  );
}
