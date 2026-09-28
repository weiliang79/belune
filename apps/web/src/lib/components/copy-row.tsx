import type { ReactNode } from "react";
import { CopyButton } from "@/lib/components/copy-button";

interface CopyRowProps {
  value: string;
  /** Wraps long, multi-line values (a shell command) instead of truncating
   *  to one line (a token, a URL). */
  multiline?: boolean;
  /** Extra controls rendered after the copy button — a Regenerate button. */
  children?: ReactNode;
}

/**
 * A read-only value with a copy button: the shared shape for "here's a
 * secret, URL, or command — shown once, or on demand." Extracted after the
 * same bg-muted/code/CopyButton block turned up independently in the
 * account tokens dialog, the deploy-hook/push-webhook rows, and the
 * Connect-an-AI-Assistant dialog.
 */
export function CopyRow({ value, multiline = false, children }: CopyRowProps) {
  return (
    <div
      className={`bg-muted flex gap-2 rounded-md px-3 py-2 ${multiline ? "items-start" : "items-center"}`}
    >
      <code
        className={`min-w-0 flex-1 font-mono text-sm break-all ${multiline ? "whitespace-pre-wrap" : ""}`}
      >
        {value}
      </code>
      <CopyButton value={value} />
      {children}
    </div>
  );
}
