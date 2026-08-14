import type { MetadataRoute } from 'next';
import { siteUrl } from '@/lib/shared';

export const dynamic = 'force-static';

// Staging stays uncrawlable until the public flip. Set NEXT_PUBLIC_ALLOW_INDEX=true
// in the production Cloudflare Pages environment to allow indexing.
export default function robots(): MetadataRoute.Robots {
  const allowIndex = process.env.NEXT_PUBLIC_ALLOW_INDEX === 'true';
  if (!allowIndex) {
    return { rules: { userAgent: '*', disallow: '/' } };
  }
  return {
    rules: { userAgent: '*', allow: '/' },
    sitemap: `${siteUrl}/sitemap.xml`,
  };
}
