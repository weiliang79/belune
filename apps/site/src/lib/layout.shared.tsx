import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { BeluneLogo } from '@/components/belune-logo';
import { appName, gitConfig } from './shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: (
        <>
          <BeluneLogo className="size-5 text-fd-primary" />
          <span className="font-semibold">{appName}</span>
        </>
      ),
    },
    githubUrl: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
  };
}
