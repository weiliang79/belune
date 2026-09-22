import { cn } from "@/lib/utils";

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      // bg-muted sits ~1.05:1 from the surrounding card/background in both
      // themes here — nearly invisible. bg-foreground/10 (already used for
      // hover/focus states in select.tsx and dropdown-menu.tsx) is
      // theme-adaptive and reads clearly against both.
      className={cn("bg-foreground/10 animate-pulse rounded-md", className)}
      {...props}
    />
  );
}

export { Skeleton };
