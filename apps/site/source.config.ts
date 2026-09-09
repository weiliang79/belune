import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import { rehypeCodeDefaultOptions } from 'fumadocs-core/mdx-plugins';
import { z } from 'zod';
import { caddyfile } from './lib/caddyfile';

// You can customize Zod schemas for frontmatter and `meta.json` here
// see https://fumadocs.dev/docs/mdx/collections
export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    // pageSchema is z.object({...}), which strips unknown keys by default
    // (Zod 4's z.core.$strip) rather than erroring or passing them through
    // — checked by reading fumadocs-core/source/schema.js, not assumed. An
    // `admin: true` frontmatter flag (written by
    // apps/site/scripts/generate-api-pages.mjs's markAdminOperations, read
    // by src/lib/source.tsx's sidebar-badge logic) would otherwise parse to
    // `undefined` silently, with no error to point at why the badge never
    // showed up.
    schema: pageSchema.extend({ admin: z.boolean().optional() }),
    postprocess: {
      includeProcessedMarkdown: true,
    },
  },
  meta: {
    schema: metaSchema,
  },
});

export default defineConfig({
  mdxOptions: {
    // Preload every language used in the docs so Shiki has the grammar at
    // build time (the default bundle is minimal and lazy-loads nothing here).
    rehypeCodeOptions: {
      ...rehypeCodeDefaultOptions,
      langs: ['bash', 'ini', 'dockerfile', 'json', 'yaml', 'sql', 'ts', 'tsx', caddyfile],
    },
  },
});
