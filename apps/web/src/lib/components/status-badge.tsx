import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const statusColor: Record<string, string> = {
  running: "bg-green-500",
  stopped: "bg-gray-500",
  deploying: "bg-yellow-500",
  building: "bg-yellow-500",
  creating: "bg-yellow-500",
  pending: "bg-yellow-500",
  failed: "bg-red-500",
};

export function StatusBadge({ status }: { status: string }) {
  return (
    <Badge variant="outline" className="gap-1.5 capitalize">
      <span
        aria-hidden="true"
        className={cn(
          "size-2 rounded-full",
          statusColor[status] ?? "bg-gray-500",
        )}
      />
      {status}
    </Badge>
  );
}
