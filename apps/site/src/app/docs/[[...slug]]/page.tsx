import { getPageImage, getPageMarkdownUrl, openapi, source } from '@/lib/source';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
  ViewOptionsPopover,
} from 'fumadocs-ui/layouts/docs/page';
import { notFound } from 'next/navigation';
import { getMDXComponents } from '@/components/mdx';
import { OpenAPIPage } from '@/components/openapi-page';
import type { Metadata } from 'next';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import { gitConfig } from '@/lib/shared';
import type { GeneratedPageProps } from 'fumadocs-openapi';

export default async function Page(props: PageProps<'/docs/[[...slug]]'>) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  const MDX = page.data.body;
  const markdownUrl = getPageMarkdownUrl(page).url;

  // Generated api/*.mdx pages (apps/site/scripts/generate-api-pages.mjs)
  // carry `_openapi` frontmatter naming which document(s) to preload —
  // <OpenAPIPage> reads doc content from the `preloaded` prop keyed by that
  // same path, not by fetching `document` itself (verified live: the plain
  // registered component throws "Cannot read properties of undefined
  // (reading 'bundled')" without it). Bound per-request here, not as a
  // module-level singleton, since preloaded content is page-specific.
  const isOpenAPIPage = Boolean(page.data._openapi);
  const openApiComponents = isOpenAPIPage
    ? {
        OpenAPIPage: async (p: GeneratedPageProps) => {
          const { preloaded } = await openapi.preloadOpenAPIPage(page);
          return <OpenAPIPage {...p} preloaded={preloaded} />;
        },
      }
    : undefined;

  return (
    <DocsPage toc={page.data.toc} full={page.data.full}>
      {/* Generated api/*.mdx pages skip the title/description/toolbar row —
          <OpenAPIPage> renders its own operation title (showTitle, set by
          generate-api-pages.mjs), so DocsTitle here would be a duplicate,
          and its two-column layout (code samples pinned to the right) reads
          better without competing chrome above it. */}
      {!isOpenAPIPage && (
        <>
          <DocsTitle>{page.data.title}</DocsTitle>
          <DocsDescription className="mb-0">{page.data.description}</DocsDescription>
          <div className="flex flex-row gap-2 items-center border-b pb-6">
            <MarkdownCopyButton markdownUrl={markdownUrl} />
            <ViewOptionsPopover
              markdownUrl={markdownUrl}
              githubUrl={`https://github.com/${gitConfig.user}/${gitConfig.repo}/blob/${gitConfig.branch}/content/docs/${page.path}`}
            />
          </div>
        </>
      )}
      <DocsBody>
        <MDX
          components={getMDXComponents({
            // this allows you to link to other pages with relative file paths
            a: createRelativeLink(source, page),
            ...openApiComponents,
          })}
        />
      </DocsBody>
    </DocsPage>
  );
}

export async function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata(props: PageProps<'/docs/[[...slug]]'>): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  return {
    title: page.data.title,
    description: page.data.description,
    openGraph: {
      images: getPageImage(page).url,
    },
  };
}
