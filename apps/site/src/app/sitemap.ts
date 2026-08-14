import type { MetadataRoute } from 'next';
import { source } from '@/lib/source';
import { siteUrl } from '@/lib/shared';

export const dynamic = 'force-static';

export default function sitemap(): MetadataRoute.Sitemap {
  const docs = source.getPages().map((page) => ({
    url: `${siteUrl}${page.url}`,
    lastModified: new Date(),
    changeFrequency: 'weekly' as const,
  }));

  return [
    { url: siteUrl, lastModified: new Date(), changeFrequency: 'weekly' },
    ...docs,
  ];
}
