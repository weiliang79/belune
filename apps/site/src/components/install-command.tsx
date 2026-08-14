'use client';

import { Check, Copy } from 'lucide-react';
import { useState } from 'react';

const COMMAND = 'curl -fsSL https://belune.dev/install.sh | bash';

export function InstallCommand() {
  const [copied, setCopied] = useState(false);

  function copy() {
    navigator.clipboard.writeText(COMMAND).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className="group flex items-center gap-3 rounded-xl border border-fd-border bg-fd-card/60 px-4 py-3 font-mono text-sm backdrop-blur">
      <span aria-hidden className="select-none text-fd-primary">
        $
      </span>
      <code className="flex-1 overflow-x-auto whitespace-nowrap text-fd-foreground">
        {COMMAND}
      </code>
      <button
        type="button"
        onClick={copy}
        aria-label={copied ? 'Copied' : 'Copy install command'}
        className="shrink-0 rounded-md p-1.5 text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fd-primary"
      >
        {copied ? (
          <Check className="size-4 text-fd-primary" />
        ) : (
          <Copy className="size-4" />
        )}
      </button>
    </div>
  );
}
