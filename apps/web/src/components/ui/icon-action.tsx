import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

// Icon-only row action with a hover/focus tooltip label. Matches the project
// services datatable action column (see service-row.tsx). Set `destructive` for
// a red delete-style action.
export function IconAction({
  label,
  onClick,
  children,
  className,
  disabled,
  destructive,
  size = "icon",
}: {
  label: string;
  onClick: () => void;
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  destructive?: boolean;
  size?: "icon" | "icon-sm";
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size={size}
            aria-label={label}
            onClick={onClick}
            disabled={disabled}
            className={cn(
              destructive &&
                "text-destructive hover:bg-destructive/10 hover:text-destructive",
              className,
            )}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipPositioner>
        <TooltipContent>{label}</TooltipContent>
      </TooltipPositioner>
    </Tooltip>
  );
}
