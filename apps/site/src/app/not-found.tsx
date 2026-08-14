import Link from 'next/link';
import { BeluneLogo } from '@/components/belune-logo';

export default function NotFound() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-8 px-6 py-32 text-center">
      <BeluneLogo className="size-12 text-fd-primary" />
      <div>
        <p className="font-mono text-sm uppercase tracking-[0.2em] text-fd-primary">
          404
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight">
          This page drifted off.
        </h1>
        <p className="mt-2 text-fd-muted-foreground">
          The page you&apos;re looking for isn&apos;t here.
        </p>
      </div>
      <div className="flex gap-3">
        <Link
          href="/"
          className="inline-flex h-11 items-center rounded-lg bg-fd-primary px-5 font-medium text-fd-primary-foreground transition-opacity hover:opacity-90"
        >
          Home
        </Link>
        <Link
          href="/docs"
          className="inline-flex h-11 items-center rounded-lg border border-fd-border px-5 font-medium transition-colors hover:bg-fd-accent"
        >
          Read the docs
        </Link>
      </div>
    </main>
  );
}
