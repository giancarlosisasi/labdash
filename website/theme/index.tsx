/**
 * labdash theme entry.
 *
 * Level 1 (CSS variables) carries the palette; level 3 (Layout slots) adds the
 * one thing the default theme has no opinion about: the wordmark in the nav.
 * Nothing is ejected — see .claude/skills/rspress-custom-theme/SKILL.md for why
 * that ordering matters when Rspress moves.
 */
import '@fontsource-variable/jetbrains-mono';
import './index.css';

import { Layout as OriginalLayout } from '@rspress/core/theme-original';

import { NavMark } from './components/Wordmark';
import { SiteFooter } from './components/SiteFooter';
import { ConsentBanner } from './components/ConsentBanner';

export * from '@rspress/core/theme-original';

export function Layout() {
  return (
    <OriginalLayout
      navTitle={<NavMark />}
      bottom={
        <>
          <SiteFooter />
          <ConsentBanner />
        </>
      }
    />
  );
}

// Re-exported for MDX pages that import explicitly. Most pages do not need to:
// these are registered as global components in rspress.config.ts.
export { Terminal } from './components/Terminal';
export { Shot } from './components/Shot';
export { Keys } from './components/Keys';
export { Palette } from './components/Palette';
export { Cards } from './components/Cards';
export { Card } from './components/Card';
export { Stats } from './components/Stats';
export { Stat } from './components/Stat';
export { Wordmark, NavMark, STANDARD_FIGLET } from './components/Wordmark';
