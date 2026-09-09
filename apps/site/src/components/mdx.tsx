import defaultMdxComponents from 'fumadocs-ui/mdx';
import type { MDXComponents } from 'mdx/types';
import { BeluneLogo } from '@/components/belune-logo';
import { Screenshot } from '@/components/screenshot';
import { Mermaid } from '@/components/mermaid';
import { OpenAPIPage } from '@/components/openapi-page';

// The generated api/*.mdx pages (apps/site/scripts/generate-api-pages.mjs)
// each reference `OpenAPIPage`/`APIPage` from `props.components`, not a
// direct import — this is what supplies it.

export function getMDXComponents(components?: MDXComponents) {
  return {
    ...defaultMdxComponents,
    Screenshot,
    BeluneLogo,
    Mermaid,
    OpenAPIPage,
    APIPage: OpenAPIPage, // legacy name the generated pages also check for
    ...components,
  } satisfies MDXComponents;
}

export const useMDXComponents = getMDXComponents;

declare global {
  type MDXProvidedComponents = ReturnType<typeof getMDXComponents>;
}
