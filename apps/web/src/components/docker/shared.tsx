import { Badge } from "@/components/ui/badge";

/** Small badge marking a resource as created and owned by the platform. */
export function ManagedBadge({ managed }: { managed: boolean }) {
  if (!managed) {
    return <span className="text-text-faint text-xs">External</span>;
  }
  return (
    <Badge variant="light" className="font-normal">
      Platform
    </Badge>
  );
}
