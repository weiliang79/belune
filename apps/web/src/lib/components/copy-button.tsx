import { useState } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "@/lib/utils/clipboard";

interface CopyButtonProps {
  value: string;
  /** Size variant — defaults to "icon" (square icon button) */
  size?: "icon" | "sm";
  className?: string;
}

/**
 * A small button that copies `value` to the clipboard and briefly shows
 * a checkmark confirmation. Uses the navigator.clipboard API with a silent
 * fallback on failure.
 */
export function CopyButton({ value, size = "icon", className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    const ok = await copyToClipboard(value);
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <Button
      variant="ghost"
      size={size === "icon" ? "icon" : "sm"}
      className={size === "icon" ? `h-7 w-7 ${className ?? ""}` : className}
      onClick={handleCopy}
      title="Copy to clipboard"
    >
      {copied ? (
        <Check className="h-3.5 w-3.5 text-green-500" />
      ) : (
        <Copy className="h-3.5 w-3.5" />
      )}
    </Button>
  );
}
