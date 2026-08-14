import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import { rehypeCodeDefaultOptions } from 'fumadocs-core/mdx-plugins';
import { caddyfile } from './lib/caddyfile';

// You can customize Zod schemas for frontmatter and `meta.json` here
// see https://fumadocs.dev/docs/mdx/collections
export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    schema: pageSchema,
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
