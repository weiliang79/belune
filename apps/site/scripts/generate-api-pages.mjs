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
import { createOpenAPI } from 'fumadocs-openapi/server';
import { generateFiles } from 'fumadocs-openapi';
import { writeFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const OUT_DIR = './content/docs/api';

// Mirrors apidocDomainOrder in apps/api/internal/handler/apidoc_generate_test.go
// (tag title -> slug) — both this script's folder names and the top-level
// meta.json's `pages` array (rewritten below) have to agree with it. Default
// slugification (lowercasing the title verbatim) produces folder names like
// "admin-—-git-provider-configs" that don't match, so this is used as the
// slugify hook instead of trying to reconcile two independent slugifiers.
const SLUG_BY_TITLE = {
  'Live Updates (WebSocket Hub)': 'live-updates',
  'Terminal Access': 'terminal',
  'Personal Access Tokens': 'tokens',
  'Session & Account': 'account',
  'Git Integrations': 'git-integrations',
  'Admin — Git Provider Configs': 'git-providers',
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

const server = createOpenAPI({
  input: ['./public/openapi.json'],
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
});

// The above also regenerates the TOP-LEVEL content/docs/api/meta.json (one
// write per schema's root entries, unconditionally, with no way to opt a
// single level out) — title/description come out blank there and, more
// importantly, `root: true` is dropped entirely, which would silently break
// the API tab out of the sidebar's tab switcher. Restored here instead of
// trying to suppress the library's own top-level write. Order comes from
// SLUG_BY_TITLE itself (written in the same order as apidocDomainOrder), not
// an alphabetical directory listing — losing that curated reading order
// would be a real regression, not just cosmetic.
const presentDirs = new Set(
  readdirSync(OUT_DIR, { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => e.name),
);
const domainSlugs = [...new Set(Object.values(SLUG_BY_TITLE))].filter((slug) => presentDirs.has(slug));

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
