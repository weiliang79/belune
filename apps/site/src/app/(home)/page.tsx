import Link from "next/link";
import {
  Activity,
  Bell,
  Boxes,
  Check,
  Database,
  FolderGit2,
  GitBranch,
  HardDrive,
  Lock,
  Minus,
  RefreshCw,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import { BeluneLogo } from "@/components/belune-logo";
import { InstallCommand } from "@/components/install-command";
import { ScreenshotImg } from "@/components/screenshot";
import { gitConfig } from "@/lib/shared";

const githubUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}`;

/** Mono eyebrow — reads like a shell prompt, encoding Belune's CLI-first nature. */
function Eyebrow({ children }: { children: React.ReactNode }) {
  return (
    <p className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-fd-primary">
      {children}
    </p>
  );
}

export default function HomePage() {
  return (
    <main className="flex flex-1 flex-col">
      <Hero />
      <Differentiators />
      <ShowBand />
      <FeatureGrid />
      <Comparison />
      <Faq />
      <Footer />
    </main>
  );
}

/* ── 1. Hero ─────────────────────────────────────────────────────────────── */
function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-fd-border">
      {/* Nocturnal "moonlight" — a soft violet crescent glow, the signature atmosphere. */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 -top-40 h-130 opacity-70"
        style={{
          background:
            "radial-gradient(60% 60% at 70% 0%, color-mix(in oklch, var(--brand) 40%, transparent), transparent 70%)",
        }}
      />
      <div className="relative mx-auto grid max-w-6xl gap-12 px-6 py-20 md:py-28 lg:grid-cols-[1.05fr_1fr] lg:items-center">
        <div>
          <span className="mb-6 inline-flex items-center gap-2 rounded-full border border-fd-border bg-fd-card/60 px-3 py-1 font-mono text-xs text-fd-muted-foreground backdrop-blur">
            <BeluneLogo className="size-3.5 text-fd-primary" />
            Open source · Apache-2.0
          </span>
          <h1 className="text-balance text-5xl font-semibold leading-[1.05] tracking-tight md:text-6xl">
            Deploy like the cloud.
          </h1>
          <p className="mt-5 max-w-xl text-balance text-lg text-fd-muted-foreground">
            Belune is an open-source, self-hosted PaaS that runs as a single Go
            binary next to Docker — git-push deploys, managed databases,
            automatic TLS, and backups you can actually restore. On your own
            server.
          </p>
          <div className="mt-8 max-w-xl">
            <InstallCommand />
            <p className="mt-2 font-mono text-xs text-fd-muted-foreground">
              Linux host with Docker. Installs in about a minute.
            </p>
          </div>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link
              href="/docs"
              className="inline-flex h-11 items-center rounded-lg bg-fd-primary px-5 font-medium text-fd-primary-foreground transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fd-primary"
            >
              Read the docs
            </Link>
            <a
              href={githubUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-11 items-center gap-2 rounded-lg border border-fd-border px-5 font-medium transition-colors hover:bg-fd-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fd-primary"
            >
              <GitBranch className="size-4" />
              GitHub
            </a>
          </div>
        </div>

        <DeployTranscript />
      </div>
    </section>
  );
}

/**
 * Illustrative deploy transcript — a deliberately stylized, real-shaped CLI
 * hero element. (Real product screenshots live in the ShowBand section below.)
 */
function DeployTranscript() {
  const lines: Array<{ tone?: "brand" | "muted" | "ok"; text: string }> = [
    { tone: "muted", text: "$ git push belune main" },
    { text: "Enumerating objects: 42, done." },
    { tone: "brand", text: "→ detected Dockerfile · building image" },
    { text: "exporting layers … done" },
    { tone: "brand", text: "→ provisioning postgres · attaching volume" },
    { tone: "brand", text: "→ issuing certificate · app.example.com" },
    { tone: "ok", text: "✓ live at https://app.example.com  (18s)" },
  ];

  return (
    <div className="rounded-2xl border border-fd-border bg-fd-card/70 shadow-2xl backdrop-blur">
      <div className="flex items-center gap-2 border-b border-fd-border px-4 py-3">
        <span className="size-3 rounded-full bg-fd-border" />
        <span className="size-3 rounded-full bg-fd-border" />
        <span className="size-3 rounded-full bg-fd-border" />
        <span className="ml-2 font-mono text-xs text-fd-muted-foreground">
          Deploy
        </span>
      </div>
      <pre className="overflow-x-auto p-5 font-mono text-[13px] leading-relaxed">
        {lines.map((line, i) => (
          <div
            key={i}
            className={
              line.tone === "brand"
                ? "text-fd-primary"
                : line.tone === "ok"
                  ? "text-emerald-500 dark:text-emerald-400"
                  : line.tone === "muted"
                    ? "text-fd-muted-foreground"
                    : "text-fd-foreground"
            }
          >
            {line.text}
          </div>
        ))}
      </pre>
    </div>
  );
}

/* ── 2. Differentiators ──────────────────────────────────────────────────── */
function Differentiators() {
  const items = [
    {
      icon: Database,
      title: "Databases treated like they matter",
      body: "Scheduled backups to S3, one-command restore, and guarded major-version upgrades that snapshot first. Not an afterthought bolted onto a container.",
    },
    {
      icon: ShieldCheck,
      title: "TLS you can see",
      body: "Live certificate status per domain — and the actual reason when issuance fails, instead of a silent 526. Caddy does the work; Belune shows it.",
    },
    {
      icon: Boxes,
      title: "One Go binary + Docker",
      body: "No Kubernetes, no control-plane sprawl, no cluster to babysit. A single binary talks to Docker on the host. Install, and you have a platform.",
    },
  ];
  return (
    <section className="mx-auto max-w-6xl px-6 py-20">
      <Eyebrow>why belune</Eyebrow>
      <h2 className="max-w-2xl text-3xl font-semibold tracking-tight">
        The parts other self-hosted PaaS tools leave to you.
      </h2>
      <div className="mt-10 grid gap-px overflow-hidden rounded-2xl border border-fd-border bg-fd-border md:grid-cols-3">
        {items.map(({ icon: Icon, title, body }) => (
          <div key={title} className="bg-fd-background p-7">
            <Icon className="size-6 text-fd-primary" />
            <h3 className="mt-4 text-lg font-medium">{title}</h3>
            <p className="mt-2 text-sm leading-relaxed text-fd-muted-foreground">
              {body}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ── 3. Show band ────────────────────────────────────────────────────────── */
function ShowBand() {
  return (
    <section className="border-y border-fd-border bg-fd-card/40">
      <div className="mx-auto max-w-6xl px-6 py-20">
        <Eyebrow>from repo to running</Eyebrow>
        <h2 className="max-w-2xl text-3xl font-semibold tracking-tight">
          Pick a template, or push your code. Belune does the rest.
        </h2>
        <div className="mt-10 grid gap-4 lg:grid-cols-3">
          <AssetFrame className="lg:col-span-2 lg:row-span-2" tall title="Deploy">
            <video
              className="block h-full w-full object-cover"
              src="/videos/deploy.mp4"
              poster="/videos/deploy-poster.jpg"
              preload="metadata"
              autoPlay
              loop
              muted
              playsInline
            />
          </AssetFrame>
          <AssetFrame title="Project Dashboard">
            <ScreenshotImg
              name="03-dashboard-home"
              alt="The Belune dashboard: projects and their services at a glance."
            />
          </AssetFrame>
          <AssetFrame title="Database Details">
            <ScreenshotImg
              name="db-overview"
              alt="A managed database's overview: connection details and external access."
            />
          </AssetFrame>
        </div>
      </div>
    </section>
  );
}

/** Browser-chrome frame wrapping a real screenshot, GIF, or video asset. */
function AssetFrame({
  children,
  className = "",
  title,
  tall = false,
}: {
  children: React.ReactNode;
  className?: string;
  title?: string;
  tall?: boolean;
}) {
  return (
    <div
      className={`flex flex-col overflow-hidden rounded-xl border border-fd-border bg-fd-background ${className}`}
    >
      <div className="flex items-center gap-3 border-b border-fd-border px-3 py-2">
        <div className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-full bg-fd-border" />
          <span className="size-2.5 rounded-full bg-fd-border" />
          <span className="size-2.5 rounded-full bg-fd-border" />
        </div>
        {title && (
          <p className="font-mono text-[10px] text-fd-muted-foreground">
            {title}
          </p>
        )}
      </div>

      <div className={tall ? "flex-1 min-h-80" : "min-h-38"}>{children}</div>
    </div>
  );
}

/* ── 4. Feature grid ─────────────────────────────────────────────────────── */
function FeatureGrid() {
  const features = [
    {
      icon: Boxes,
      title: "App templates",
      body: "One-click catalog of ready-to-run apps.",
    },
    {
      icon: GitBranch,
      title: "Push to deploy",
      body: "Git push builds and ships automatically.",
    },
    {
      icon: Database,
      title: "Managed databases",
      body: "Postgres, MySQL, Redis, and more.",
    },
    {
      icon: HardDrive,
      title: "Volumes & mounts",
      body: "Persistent data and config files.",
    },
    {
      icon: Bell,
      title: "Notifications",
      body: "Route events to Discord, Slack, email, and webhooks.",
    },
    {
      icon: Terminal,
      title: "Web terminal",
      body: "A shell into any container from the browser.",
    },
    {
      icon: Activity,
      title: "Metrics & logs",
      body: "Live resource graphs and leveled logs.",
    },
    {
      icon: FolderGit2,
      title: "Rebuild from commit",
      body: "Rebuild or reload from any pinned commit.",
    },
    {
      icon: Lock,
      title: "Certificates",
      body: "Automatic HTTPS with visible status.",
    },
  ];
  return (
    <section className="mx-auto max-w-6xl px-6 py-20">
      <Eyebrow>everything included</Eyebrow>
      <h2 className="max-w-2xl text-3xl font-semibold tracking-tight">
        A whole platform, not a deploy script.
      </h2>
      <div className="mt-10 grid gap-px overflow-hidden rounded-2xl border border-fd-border bg-fd-border sm:grid-cols-2 lg:grid-cols-3">
        {features.map(({ icon: Icon, title, body }) => (
          <div key={title} className="bg-fd-background p-6">
            <div className="flex items-center gap-3">
              <Icon className="size-5 text-fd-primary" />
              <h3 className="font-medium">{title}</h3>
            </div>
            <p className="mt-2 text-sm text-fd-muted-foreground">{body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ── 5. Comparison ───────────────────────────────────────────────────────── */
function Comparison() {
  const cols = ["Belune", "Coolify", "Dokploy", "CapRover"];
  // yes | no | soon — kept conservative and honest; re-verify competitor state at launch.
  const rows: Array<{ label: string; cells: Array<"yes" | "no" | "soon"> }> = [
    { label: "Single-binary control plane", cells: ["yes", "no", "no", "no"] },
    {
      label: "Per-domain TLS status & failure reasons",
      cells: ["yes", "no", "no", "no"],
    },
    {
      label: "Managed DB backups & restore",
      cells: ["yes", "yes", "yes", "no"],
    },
    {
      label: "Guarded major-version DB upgrades",
      cells: ["yes", "no", "no", "no"],
    },
    { label: "Web terminal", cells: ["yes", "yes", "yes", "no"] },
    { label: "Multi-server", cells: ["soon", "yes", "yes", "yes"] },
    { label: "Docker Compose import", cells: ["soon", "yes", "yes", "no"] },
  ];
  return (
    <section className="border-y border-fd-border bg-fd-card/40">
      <div className="mx-auto max-w-6xl px-6 py-20">
        <Eyebrow>the honest table</Eyebrow>
        <h2 className="max-w-2xl text-3xl font-semibold tracking-tight">
          How Belune compares — including where it doesn&apos;t win yet.
        </h2>
        <div className="mt-10 overflow-x-auto rounded-2xl border border-fd-border">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-fd-border bg-fd-card/60">
                <th className="p-4 text-left font-medium">Feature</th>
                {cols.map((c, i) => (
                  <th
                    key={c}
                    className={`p-4 text-center font-medium ${i === 0 ? "text-fd-primary" : "text-fd-muted-foreground"}`}
                  >
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.label}
                  className="border-b border-fd-border last:border-0"
                >
                  <td className="p-4 text-left">{row.label}</td>
                  {row.cells.map((cell, i) => (
                    <td key={i} className="p-4 text-center">
                      <Cell value={cell} highlight={i === 0} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-4 font-mono text-xs text-fd-muted-foreground">
          Point-in-time comparison; competitors ship fast — verify against their
          current docs. &ldquo;Soon&rdquo; = on the Belune roadmap, and free
          when it lands.
        </p>
      </div>
    </section>
  );
}

function Cell({
  value,
  highlight,
}: {
  value: "yes" | "no" | "soon";
  highlight: boolean;
}) {
  if (value === "yes")
    return (
      <Check
        className={`mx-auto size-4 ${highlight ? "text-fd-primary" : "text-fd-foreground"}`}
      />
    );
  if (value === "soon")
    return (
      <span className="font-mono text-xs text-fd-muted-foreground">soon</span>
    );
  return <Minus className="mx-auto size-4 text-fd-muted-foreground/50" />;
}

/* ── 6. FAQ ──────────────────────────────────────────────────────────────── */
function Faq() {
  const items = [
    {
      q: "Is it really free?",
      a: "Yes. Belune is Apache-2.0 licensed. Everything an individual needs — including audit logs — is and will remain free. Any future paid tier targets organization-only needs like SAML/SCIM.",
    },
    {
      q: "Does it support Docker Compose?",
      a: "Not yet. Compose import is planned after launch as a way to bootstrap a project from an existing compose file — a starting point, not a runtime format Belune passes through.",
    },
    {
      q: "What do I need to run it?",
      a: "A Linux host with Docker installed. Belune talks to the Docker socket to build images and run your apps and databases. No Kubernetes, no external control plane.",
    },
    {
      q: "How safe is my data?",
      a: "Databases back up on a schedule to S3-compatible storage, restores are one command, and major-version upgrades snapshot before they touch anything. Secrets are encrypted at rest.",
    },
    {
      q: "What does the version number mean?",
      a: "Belune is pre-1.0. A minor release (0.x) may include breaking changes; a patch is always safe to apply. Back up before upgrading — Belune makes that a one-liner.",
    },
  ];
  return (
    <section className="mx-auto max-w-3xl px-6 py-20">
      <Eyebrow>questions</Eyebrow>
      <h2 className="text-3xl font-semibold tracking-tight">
        Straight answers.
      </h2>
      <dl className="mt-10 divide-y divide-fd-border border-y border-fd-border">
        {items.map(({ q, a }) => (
          <div key={q} className="py-6">
            <dt className="font-medium">{q}</dt>
            <dd className="mt-2 text-sm leading-relaxed text-fd-muted-foreground">
              {a}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/* ── 7. Footer ───────────────────────────────────────────────────────────── */
function Footer() {
  return (
    <footer className="border-t border-fd-border">
      <div className="mx-auto flex max-w-6xl flex-col gap-6 px-6 py-12 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2.5">
          <BeluneLogo className="size-5 text-fd-primary" />
          <span className="font-semibold">Belune</span>
          <span className="ml-1 font-mono text-xs text-fd-muted-foreground">
            Deploy like the cloud.
          </span>
        </div>
        <nav className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-fd-muted-foreground">
          <Link href="/docs" className="hover:text-fd-foreground">
            Docs
          </Link>
          <a
            href={githubUrl}
            className="hover:text-fd-foreground"
            target="_blank"
            rel="noreferrer"
          >
            GitHub
          </a>
          <a
            href={`${githubUrl}/releases`}
            className="hover:text-fd-foreground"
            target="_blank"
            rel="noreferrer"
          >
            Releases
          </a>
          <a
            href={`${githubUrl}/blob/${gitConfig.branch}/LICENSE`}
            className="hover:text-fd-foreground"
            target="_blank"
            rel="noreferrer"
          >
            Apache-2.0
          </a>
        </nav>
      </div>
    </footer>
  );
}
