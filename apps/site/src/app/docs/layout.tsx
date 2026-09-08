import { getSidebarTree } from '@/lib/source';
import { DocsLayout } from 'fumadocs-ui/layouts/notebook';
import { baseOptions } from '@/lib/layout.shared';
import { Braces, LayoutDashboard } from 'lucide-react';
import type { LayoutTab } from 'fumadocs-ui/layouts/shared';
import type { ReactNode } from 'react';

// Icons keyed by meta.json's own `title` (getLayoutTabs sets LayoutTab.title
// to the root folder's `name`, i.e. its meta.json title) rather than by path
// or slug — reads directly off the same field a human editing meta.json
// would recognize. Deliberately NOT in meta.json itself: a JSON `icon` field
// needs an icon resolver wired into source.ts's loader(), which doesn't
// exist, and — for the API tab specifically — would be a third thing
// apidoc_generate_test.go has to own and re-emit every regeneration,
// alongside Title and Description. Icons are presentation, kept here;
// content (title, description) stays in each root's meta.json.
const tabIcons: Record<string, ReactNode> = {
  Core: <LayoutDashboard />,
  API: <Braces />,
};

function withIcon(tab: LayoutTab): LayoutTab {
  const icon = typeof tab.title === 'string' ? tabIcons[tab.title] : undefined;
  return icon ? { ...tab, icon } : tab;
}

// `layouts/notebook`, not `layouts/docs`: the two differ in where the tab
// dropdown sits in the sidebar (notebook puts it directly under the logo,
// above search; docs puts it below search) — baked into each layout's own
// sidebar slot, not a prop, so this can't be fixed on `layouts/docs`. The
// paired page component (`src/app/docs/[[...slug]]/page.tsx`) has to import
// from `fumadocs-ui/layouts/notebook/page` too — DocsPage throws at runtime
// if it's rendered under the wrong DocsLayout.
//
// `tabs={{ transform: withIcon }}` (not a precomputed LayoutTab[]) still lets
// DocsLayout derive title/description/url from the `root: true` folders
// (Core, API) that getSidebarTree() leaves intact — only the icon is added
// on top, since that's the one field with nowhere else to live. A
// hand-maintained LayoutTab[] here would be a second, hand-maintained place
// the title/description/url could drift from meta.json's own fields.
//
// `tabMode="sidebar"` is notebook's default (unlike `layouts/docs`, whose
// prop is `"auto" | "top"` — notebook's is `"sidebar" | "navbar"`, a
// different enum, not a renamed version of the same one) — named explicitly
// rather than left implicit. The intro page at `/docs` itself sits outside
// both roots, so neither tab shows active there — that's expected, not a bug.
export default function Layout({ children }: LayoutProps<'/docs'>) {
  return (
    <DocsLayout
      tree={getSidebarTree()}
      tabs={{ transform: withIcon }}
      tabMode="sidebar"
      {...baseOptions()}
    >
      {children}
    </DocsLayout>
  );
}
