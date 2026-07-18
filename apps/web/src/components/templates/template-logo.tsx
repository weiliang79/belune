import { useState } from "react";
import { Package2 } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * App logo for a template, with a lucide fallback when no logo_url is set or the
 * image fails to load. Shared by the catalog card and the wizard.
 */
export function TemplateLogo({
  logoUrl,
  className,
}: {
  logoUrl?: string;
  className?: string;
}) {
  const [failed, setFailed] = useState(false);
  if (logoUrl && !failed) {
    return (
      <img
        src={logoUrl}
        alt=""
        loading="lazy"
        onError={() => setFailed(true)}
        className={cn("size-10 shrink-0 rounded-lg object-contain", className)}
      />
    );
  }
  return (
    <div
      className={cn(
        "bg-elev text-text-muted grid size-10 shrink-0 place-items-center rounded-lg",
        className,
      )}
    >
      <Package2 aria-hidden="true" className="size-1/2" />
    </div>
  );
}
