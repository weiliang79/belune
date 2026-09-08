import { getSidebarTree } from '@/lib/source';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions } from '@/lib/layout.shared';

// `tabs={{}}` (empty options, not a precomputed list) lets DocsLayout derive
// the tab switcher itself from the `root: true` folders (Core, API) that
// getSidebarTree() now leaves intact — a hand-maintained LayoutTab[] here
// would be a second place that list could drift from meta.json's own root
// flags. `tabMode="auto"` renders the switcher as a sidebar dropdown rather
// than a top tab bar. The intro page at `/docs` itself sits outside both
// roots, so neither tab shows active there — that's expected, not a bug.
export default function Layout({ children }: LayoutProps<'/docs'>) {
  return (
    <DocsLayout tree={getSidebarTree()} tabs={{}} tabMode="auto" {...baseOptions()}>
      {children}
    </DocsLayout>
  );
}
