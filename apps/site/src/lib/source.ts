import { docs } from 'collections/server';
import { loader } from 'fumadocs-core/source';
import { docsContentRoute, docsImageRoute, docsRoute } from './shared';

// See https://fumadocs.dev/docs/headless/source-api for more info
export const source = loader({
  baseUrl: docsRoute,
  source: docs.toFumadocsSource(),
  plugins: [],
});

type PageTreeRoot = ReturnType<typeof source.getPageTree>;
type PageTreeNode = PageTreeRoot['children'][number];

// Render each top-level section folder (Server, Project, Application, Database)
// as a non-collapsible separator group — like the "Overview" group — instead of a
// collapsible folder. Only the sidebar tree changes: the pages keep their own
// nested URLs (`/docs/server/git`, …), so routing is unaffected. Nested folders
// (e.g. Application → Frameworks) stay collapsible.
export function getSidebarTree(): PageTreeRoot {
  const tree = source.getPageTree();
  const children: PageTreeNode[] = [];

  for (const node of tree.children) {
    if (node.type === 'folder') {
      children.push({ type: 'separator', name: node.name });
      if (node.index) children.push(node.index);
      children.push(...node.children);
    } else {
      children.push(node);
    }
  }

  return { ...tree, children };
}

export function getPageImage(page: (typeof source)['$inferPage']) {
  const segments = [...page.slugs, 'image.png'];

  return {
    segments,
    url: `${docsImageRoute}/${segments.join('/')}`,
  };
}

export function getPageMarkdownUrl(page: (typeof source)['$inferPage']) {
  const segments = [...page.slugs, 'content.md'];

  return {
    segments,
    url: `${docsContentRoute}/${segments.join('/')}`,
  };
}

export async function getLLMText(page: (typeof source)['$inferPage']) {
  const processed = await page.data.getText('processed');

  return `# ${page.data.title} (${page.url})

${processed}`;
}
