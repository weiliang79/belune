import type { Metadata } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';
import { Provider } from '@/components/provider';
import { appName, siteDescription, siteUrl } from '@/lib/shared';
import './global.css';

const geist = Geist({
  subsets: ['latin'],
  variable: '--font-geist',
});

const geistMono = Geist_Mono({
  subsets: ['latin'],
  variable: '--font-geist-mono',
});

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: `${appName} — Deploy like the cloud.`,
    template: `%s — ${appName}`,
  },
  description: siteDescription,
  openGraph: {
    type: 'website',
    siteName: appName,
    url: siteUrl,
  },
  twitter: {
    card: 'summary_large_image',
  },
  // Staging stays invisible to crawlers until the public flip. Set
  // NEXT_PUBLIC_ALLOW_INDEX=true in the production CF Pages env to allow indexing.
  robots:
    process.env.NEXT_PUBLIC_ALLOW_INDEX === 'true'
      ? undefined
      : { index: false, follow: false },
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html
      lang="en"
      className={`${geist.variable} ${geistMono.variable}`}
      suppressHydrationWarning
    >
      <body className="flex flex-col min-h-screen font-sans">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
