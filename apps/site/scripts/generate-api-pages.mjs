// Generates one page PER OPERATION under content/docs/api/<domain>/, grouped
// into per-domain folders — Fumadocs' own default OpenAPI page shape
// (https://fumadocs.dev/docs/openapi/planets/getAllData: each operation is
// its own sidebar entry with a method badge, tags become folder groups), not
// the one-page-per-domain-with-everything-stacked shape this generator
// shipped with first. That first shape came from over-applying "keep the
// domain grouping" — the fix keeps the grouping as FOLDERS instead of
// flattening every operation onto one page.
//
// Consumes apps/site/public/openapi.json, which
// apps/api/internal/handler/apidoc_generate_test.go writes; run this AFTER
// that generator, never standalone (see task generate:api-docs).
//
// PAGES ARE A FILTERED VIEW OF THE SPEC, NOT A MIRROR OF IT. This reference's
// audience is people scripting with a personal access token — an operation
// no token can ever call, at any scope (RequireSession; see
// middleware.RequireSession and the "What a Token Can Never Do" section of
// access.mdx, which already documents the boundary in prose), is a dead end
// in that reference, not useful content. public/openapi.json itself is NOT
// filtered: it's the complete, published-at-a-stable-URL machine artifact
// third-party tooling depends on, its security block already states
// session-only correctly, and TestGenerateAPIReference's own invariant
// (every RequireSession route emits no pat* scheme) has nothing to check if
// those operations were removed from it. Only the human-facing page set
// (and the meta.json files describing it) is filtered, by cloning the
// already-generated spec in memory and feeding fumadocs-openapi that clone
// instead of the file on disk — decided by the user 2026-09-09, after
// rejecting a redundant "dashboard-only endpoints" table (access.mdx's
// prose already covers exactly this set).
//
// PREVIEW: the two Git domains nest under a shared "Git" separator — see
// the block near the bottom, past the top-level meta.json write. The one
// thing that made this unsafe to merge on its own — "Admin — Git Provider
// Configs" losing its admin-audience signal in the rename to "Provider
// Configs" — is fixed by the sidebar's "Admin only" badge (see
// markAdminOperations below and src/lib/source.tsx); the nesting shape
// itself is still the user's call to finalize.
import { createOpenAPI } from 'fumadocs-openapi/server';
import { generateFiles } from 'fumadocs-openapi';
import { writeFileSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';

const OUT_DIR = './content/docs/api';
const SPEC_PATH = './public/openapi.json';

// Mirrors apidocDomainOrder in apps/api/internal/handler/apidoc_generate_test.go
// (tag title -> slug) — both this script's folder names and the top-level
// meta.json's `pages` array (rewritten below) have to agree with it. Default
// slugification (lowercasing the title verbatim) produces folder names like
// "admin-—-git-provider-configs" that don't match, so this is used as the
// slugify hook instead of trying to reconcile two independent slugifiers.
// A domain can legitimately end up with zero surviving pages once
// isPatCallable below filters it (e.g. "Terminal Access" — both of its
// operations are session-only) — handled structurally, not listed here: see
// the presentDirs filter at the bottom.
const SLUG_BY_TITLE = {
  'Live Updates (WebSocket Hub)': 'live-updates',
  'Terminal Access': 'terminal',
  'Personal Access Tokens': 'tokens',
  'Session & Account': 'account',
  // Slashes here are deliberate — see the "Nest the two Git domains" block
  // near the bottom, which is the only place that depends on it.
  'Git Integrations': 'git/integrations',
  'Admin — Git Provider Configs': 'git/providers',
  'App Templates': 'templates',
  'Deploy & Lifecycle Actions': 'deploy-actions',
  Metrics: 'metrics',
  'Admin — Metrics & Live Streams': 'admin-metrics',
  'Preview Environments': 'previews',
  Deployments: 'deployments',
  'Logs & Request Traces': 'logs',
  'Domains & TLS': 'domains',
  'Application Volumes & Backups': 'volumes',
  'File Mounts': 'file-mounts',
  'Environment Variables': 'env',
  'Databases & Backups': 'databases',
  Applications: 'applications',
  'Stats & Notifications': 'stats-notifications',
  Projects: 'projects',
  'Admin — Users & Invitations': 'admin-users',
  'Admin — Platform Backups': 'admin-backups',
  'Admin — Platform': 'admin-platform',
  Uncategorized: 'uncategorized',
};

function slugifyTag(name) {
  const slug = SLUG_BY_TITLE[name];
  if (!slug) {
    throw new Error(
      `generate-api-pages: no slug mapped for tag "${name}" — add it to SLUG_BY_TITLE, keeping it in sync with apidocDomainOrder in apidoc_generate_test.go`,
    );
  }
  return slug;
}

// PascalCase (the operationId, which apidoc_generate_test.go sets to the Go
// handler's own name, e.g. "ListBackupDestinations") -> kebab-case, matching
// this site's URL convention. The default name algorithm uses operationId
// verbatim with no slugify pass, which would otherwise ship
// /docs/api/databases/ListBackupDestinations.
//
// Acronym-aware in the same way apidocTitleFromOperationID is on the Go
// side (a single boundary pass alone turns "DeleteAPIToken" into
// "delete-apitoken", not "delete-api-token" — found live, in the actual
// generated filenames, not predicted up front): a lowercase-to-uppercase
// transition is always a word boundary, but a RUN of uppercase letters
// stays together except at its own last letter, where the next word
// actually starts.
function kebabCase(name) {
  const s = name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').replace(/([A-Z]+)([A-Z][a-z])/g, '$1-$2');
  return s.toLowerCase();
}

// topSegment reduces a SLUG_BY_TITLE value to the TOP-LEVEL directory it
// actually lands in on disk — identity for a plain slug ("tokens"), the
// first path component for a nested one ("git/integrations" -> "git").
// Both the OWNED sweep and the top-level meta.json's pages array need this:
// they operate on/list top-level directory names, and a slug containing a
// slash no longer equals the directory it produces.
function topSegment(slug) {
  return slug.split('/')[0];
}

// isPatCallable is THE single predicate for "does this operation belong in
// the human-facing reference" — derived from the same structural fact
// TestAPIDocAllMarshalerTypesHandled's sibling guard checks on the Go side
// (every RequireSession route's security carries no pat* scheme), not a
// hand-maintained path list that would rot the way destroy_boundary_test.go's
// own comments warn a resource list would. Kept in exactly one place on
// purpose: the user is still reviewing which operations should actually be
// excluded, and any future exception (an operation worth keeping despite
// being session-gated) should be one documented line added here, not a
// change threaded through the generator or the tag/domain machinery.
function isPatCallable(operation) {
  return (operation.security ?? []).some((requirement) => Object.keys(requirement).some((scheme) => scheme.startsWith('pat')));
}

// isAdminGated is THE single predicate the sidebar's "Admin only" badges
// derive from (src/lib/source.tsx reads it back off each page's own
// frontmatter, not the spec directly — see markAdminOperations below for
// why) — same "derive it, don't hand-list it" reasoning as isPatCallable.
// apidocSecurity ANDs adminRole into every alternative uniformly when
// RequireRole gates a route, so checking any one alternative is enough.
function isAdminGated(operation) {
  return (operation.security ?? []).some((requirement) => 'adminRole' in requirement);
}

const fullSpec = JSON.parse(readFileSync(SPEC_PATH, 'utf8'));
const pagesSpec = structuredClone(fullSpec);
for (const [path, methods] of Object.entries(pagesSpec.paths)) {
  for (const method of Object.keys(methods)) {
    if (!isPatCallable(methods[method])) delete methods[method];
  }
  if (Object.keys(methods).length === 0) delete pagesSpec.paths[path];
}

// generateFiles only ever mkdir+writeFiles — it never deletes, so a
// operation that WAS generated on a previous run and is filtered out on this
// one (every operation in a shrinking or now-empty domain, e.g. "Terminal
// Access") would otherwise survive on disk as an orphan: still on the
// filesystem, still routable, just no longer linked from anywhere — the same
// class of bug the top-level meta.json write once had to guard against.
//
// Scoped to OWNED (every slug SLUG_BY_TITLE can produce) rather than every
// directory under OUT_DIR — deliberately, not for convenience: "every
// directory here is generator output" is the same allowlist-shaped
// assumption that already bit the Go side once (a hardcoded written :=
// {"index.mdx", "access.mdx", "meta.json"} would have silently deleted
// access.mdx on the directory rename; the fix there was marker-based, not a
// widened allowlist, specifically so the class wouldn't come back). A
// hand-written subfolder under content/docs/api/ is plausible — access.mdx
// already proves hand-written content lives in this tab — and nothing marks
// one as safe to keep, so sweeping "everything" would silently delete it on
// the next regeneration with no warning and no trace beyond the deletion.
// Restricting to OWNED still empties a domain that shrinks to nothing
// (terminal stays in SLUG_BY_TITLE even at zero operations, so its stale
// folder is still swept — the actual case this cleanup exists for) while
// leaving anything with another name alone. Don't widen this back to "all
// directories."
//
// topSegment matters here specifically because of git/integrations and
// git/providers: OWNED has to contain "git" (what actually appears as a
// top-level directory under OUT_DIR), not the full slug — a set built from
// the raw slug values would never match "git", the sweep would silently
// stop cleaning that whole subtree, and a stale page from a deleted
// operation would survive as exactly the orphan class this sweep exists to
// prevent. Checked, not assumed: see the "Nest the two Git domains" block.
const OWNED = new Set(Object.values(SLUG_BY_TITLE).map(topSegment));
for (const e of readdirSync(OUT_DIR, { withFileTypes: true })) {
  if (e.isDirectory() && OWNED.has(e.name)) rmSync(join(OUT_DIR, e.name), { recursive: true, force: true });
}

// markAdminOperations is generateFiles's beforeWrite hook (documented as
// "can add/change/remove output files before writing to file system") — the
// seam used to inject an `admin: true` frontmatter flag onto every
// admin-gated operation's page, WITHOUT threading a second lookup through
// fumadocs-openapi's own template generation (which has no option for
// extra frontmatter fields). Each generated .mdx already embeds the
// operation it covers as `operations={[{"path":...,"method":...}]}` in its
// body (regex-extracted here rather than mapped through
// generateFilesOnly's internal per-schema `generated`/`generatedEntries`
// context, which is keyed by schema id and entry structure this script
// doesn't otherwise need to know) — resolving that back against pagesSpec
// (the same filtered clone generateFiles itself ran against, so every
// operation a file exists for is guaranteed present in it) gives the
// answer. `admin` is a plain top-level frontmatter key, inserted right
// after the `title:` line — always the first line, always a single line
// (never folds, unlike `description`), so anchoring there is unambiguous.
//
// Why frontmatter and not a wired-through prop: src/lib/source.tsx builds
// the SIDEBAR TREE from page data (source.getPages()), which has no
// `security` field at all — the spec never reaches that far. Emitting the
// flag onto each generated page keeps openapi.json the single source of
// truth while still making it visible where the sidebar-tree code needs
// it. source.config.ts's pageSchema has to declare `admin` too, or Zod
// silently strips it (checked by reading the schema, not assumed) — see
// the `admin: z.boolean().optional()` extension there.
function markAdminOperations(files) {
  for (const file of files) {
    if (!file.path.endsWith('.mdx')) continue;
    const match = file.content.match(/operations=\{\[\{"path":"([^"]+)","method":"([^"]+)"\}\]\}/);
    if (!match) continue;
    const [, path, method] = match;
    const operation = pagesSpec.paths[path]?.[method];
    if (operation && isAdminGated(operation)) {
      file.content = file.content.replace(/^(---\ntitle: .*\n)/, '$1admin: true\n');
    }
  }
}

// createOpenAPI's `input` accepts an in-memory document (SchemaRecord) as an
// alternative to a file path — verified by reading fumadocs-openapi's own
// loader (server/index.js's getSchema, and @fumadocs/api-docs's bundle(),
// which explicitly branches on `typeof input !== 'string'`) rather than
// assumed from the .d.ts alone. Feeding it the filtered clone here, instead
// of writing a second file to disk, keeps SPEC_PATH itself untouched by this
// script — it only ever reads that file.
//
// The SchemaRecord key MUST be SPEC_PATH itself, not an arbitrary id: each
// generated .mdx embeds it verbatim as <Comp document="...">, and that value
// is looked up again at request/build time against src/lib/source.tsx's own,
// entirely separate `createOpenAPI({ input: ['./public/openapi.json'] })` —
// found by an actual `npm run build` failure ("Failed to resolve input:
// openapi") when this was first written with an arbitrary key, not
// predicted. The two loaders never share state — this one's filtered
// in-memory clone only ever decides which pages get generated and what
// `operations` list lands in each one's frontmatter; every operation that
// survives filtering is resolved again at runtime from the real, complete
// file, which still contains it unchanged.
const server = createOpenAPI({
  input: { [SPEC_PATH]: pagesSpec },
});

await generateFiles({
  input: server,
  output: OUT_DIR,
  per: 'operation',
  groupBy: 'tag',
  slugify: slugifyTag,
  // Regular function, not an arrow one — `name` is invoked as
  // `nameFn.call(builder, entry)` (see fumadocs-openapi's own default
  // algorithm), so `this.document` is how the bundled spec is reached to
  // look up the operationId. Same lookup the library's own default v2
  // algorithm does; only the kebab-case pass on the result is new.
  name(output) {
    const operation = this.document.paths[output.item.path][output.item.method];
    if (!operation.operationId) {
      throw new Error(`generate-api-pages: no operationId for ${output.item.method} ${output.item.path}`);
    }
    return kebabCase(operation.operationId);
  },
  meta: { folderStyle: 'folder' },
  beforeWrite: markAdminOperations,
});

// The above also regenerates the TOP-LEVEL content/docs/api/meta.json (one
// write per schema's root entries, unconditionally, with no way to opt a
// single level out) — title/description come out blank there and, more
// importantly, `root: true` is dropped entirely, which would silently break
// the API tab out of the sidebar's tab switcher. Restored here instead of
// trying to suppress the library's own top-level write. Order comes from
// SLUG_BY_TITLE itself (written in the same order as apidocDomainOrder), not
// an alphabetical directory listing — losing that curated reading order
// would be a real regression, not just cosmetic. presentDirs.has(slug) also
// does double duty as the empty-domain guard: a domain every one of whose
// operations isPatCallable filtered out (e.g. "Terminal Access") never gets
// a folder from generateFiles in the first place — group() in
// fumadocs-openapi's preset only creates a tag's group when an operation
// still references that tag — so it's excluded here the same way a domain
// with zero routes always was, no special-casing needed.
const presentDirs = new Set(
  readdirSync(OUT_DIR, { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => e.name),
);
// topSegment collapses git/integrations and git/providers to one "git"
// entry — the Set dedupes it to a single occurrence, at the position of
// whichever one SLUG_BY_TITLE lists first, which is exactly "replace the
// two entries with the single entry git, in the same position."
const domainSlugs = [...new Set(Object.values(SLUG_BY_TITLE).map(topSegment))].filter((slug) => presentDirs.has(slug));

writeFileSync(
  join(OUT_DIR, 'meta.json'),
  JSON.stringify(
    {
      title: 'API',
      description: "Authenticate and script against Belune's HTTP API.",
      root: true,
      pages: ['index', 'access', ...domainSlugs],
    },
    null,
    2,
  ) + '\n',
);

// PREVIEW, not the final shape (2026-09-10) — nests "Git Integrations" and
// "Admin — Git Provider Configs" under a shared "Git" separator, so the
// user can see it before deciding. flattenSections in src/lib/source.ts
// flattens exactly ONE level inside a root tab — checked by reading it, not
// assumed — so a git/ folder containing integrations/ and providers/
// collapses to a "Git" separator with the two as collapsible dropdowns
// underneath, for free; the sidebar needs no changes for this.
//
// The slash embedded in SLUG_BY_TITLE's two "git/..." values already made
// slugifyTag place their operations at git/integrations/*.mdx and
// git/providers/*.mdx (path.join tolerates a slash inside one argument —
// verified against the actual output below, not assumed) and made
// generateFiles's own meta:{folderStyle:'folder'} write correct
// git/integrations/meta.json and git/providers/meta.json files, with
// correct `pages` arrays — just with the ORIGINAL full tag names as their
// titles, and no git/meta.json, since fumadocs-openapi has no concept of
// these two groups sharing a "git" parent (that would need tag.parent in
// the spec, a heavier change out of scope for a preview). Both gaps are
// closed here: read each child meta.json back, replace only its title
// (CHILD_TITLES is a two-entry, hand-written map on purpose — this isn't a
// general nesting mechanism, just this one preview), and synthesize the
// parent's.
//
// "Admin — Git Provider Configs" carried its admin-only audience in its
// title; renaming it to "Provider Configs" here would have lost that signal
// on its own. Restored by the "Admin only" sidebar badge in
// src/lib/source.tsx (Folder.name/Separator.name/Item.name are all
// ReactNode in fumadocs-core — same technique the tab icons already use),
// computed there from each page's `admin` frontmatter (markAdminOperations
// above), not hand-listed here.
if (presentDirs.has('git')) {
  const CHILD_TITLES = { integrations: 'Integrations', providers: 'Provider Configs' };
  const gitChildren = readdirSync(join(OUT_DIR, 'git'), { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => e.name);

  for (const child of gitChildren) {
    const metaPath = join(OUT_DIR, 'git', child, 'meta.json');
    const meta = JSON.parse(readFileSync(metaPath, 'utf8'));
    meta.title = CHILD_TITLES[child] ?? meta.title;
    writeFileSync(metaPath, JSON.stringify(meta, null, 2));
  }

  writeFileSync(
    join(OUT_DIR, 'git', 'meta.json'),
    JSON.stringify(
      {
        title: 'Git',
        pages: ['integrations', 'providers'].filter((c) => gitChildren.includes(c)),
      },
      null,
      2,
    ),
  );
}
