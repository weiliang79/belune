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

// Render each SECTION folder (Server, Project, Application, Database under Core;
// nothing yet under API) as a non-collapsible separator group — like the
// "Overview" group — instead of a collapsible folder. Only the sidebar tree
// changes: the pages keep their own nested URLs (`/docs/core/server/git`, …),
// so routing is unaffected. Doubly-nested folders (e.g. Application →
// Frameworks) stay collapsible — this only flattens one level.
function flattenSections(nodes: PageTreeNode[]): PageTreeNode[] {
  const out: PageTreeNode[] = [];
  for (const node of nodes) {
    if (node.type === 'folder') {
      out.push({ type: 'separator', name: node.name });
      if (node.index) out.push(node.index);
      out.push(...node.children);
    } else {
      out.push(node);
    }
  }
  return out;
}

// Two `root: true` folders (Core, API — meta.json's "root" flag) exist so
// DocsLayout can render them as sidebar tabs (see `tabs` in layout.tsx): a
// root folder must stay a folder node, intact, for `getLayoutTabs()` to find
// it and for DocsLayout to scope the sidebar to whichever one is active. The
// flattening above therefore can't run at the top level like it used to when
// there was only one section total — it has to run INSIDE each root's own
// children instead, so "Server"/"Project"/etc. still render as flat groups
// once you're inside the Core tab, rather than as a nested collapsible folder.
export function getSidebarTree(): PageTreeRoot {
  const tree = source.getPageTree();
  const children: PageTreeNode[] = [];

  for (const node of tree.children) {
    if (node.type === 'folder' && node.root) {
      children.push({ ...node, children: flattenSections(node.children) });
    } else if (node.type === 'folder') {
      children.push(...flattenSections([node]));
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
