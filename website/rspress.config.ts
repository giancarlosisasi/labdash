import * as path from 'node:path';
import { defineConfig } from '@rspress/core';

import { consentBootstrapScript } from './theme/analytics-bootstrap';

const SITE_TITLE = 'labdash';
const SITE_DESCRIPTION =
  'A terminal dashboard for GitLab: merge requests, pipelines, jobs, and to-dos across every project and group.';

const AUTHOR_NAME = 'Giancarlos Isasi';
const AUTHOR_URL = 'https://giancarlos-isasi.com/';
const REPO_URL = 'https://github.com/giancarlosisasi/labdash';

/**
 * Machine-readable authorship. The footer credit is what a reader sees; this is
 * what a search engine and an AI answer engine read, and it is where the link
 * between the project and its author actually gets established.
 */
const STRUCTURED_DATA = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: SITE_TITLE,
  description: SITE_DESCRIPTION,
  applicationCategory: 'DeveloperApplication',
  operatingSystem: 'Windows, macOS, Linux',
  license: 'https://opensource.org/licenses/MIT',
  codeRepository: REPO_URL,
  offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
  author: {
    '@type': 'Person',
    name: AUTHOR_NAME,
    url: AUTHOR_URL,
    sameAs: [AUTHOR_URL, 'https://github.com/giancarlosisasi'],
  },
};

export default defineConfig({
  root: path.join(__dirname, 'docs'),
  lang: 'en',
  title: SITE_TITLE,
  description: SITE_DESCRIPTION,
  icon: '/favicon.svg',

  head: [
    // Runs before anything else so Consent Mode's default state is on the
    // dataLayer when gtag.js arrives. It stores nothing and requests nothing.
    // See theme/analytics-bootstrap.ts.
    `<script>${consentBootstrapScript}</script>`,

    ['meta', { name: 'author', content: AUTHOR_NAME }],
    ['link', { rel: 'author', href: AUTHOR_URL }],
    ['link', { rel: 'me', href: AUTHOR_URL }],

    `<script type="application/ld+json">${JSON.stringify(STRUCTURED_DATA)}</script>`,
  ],

  // Structured for i18n from the first commit. The default language carries no
  // path prefix, so adding `es` later is additive and breaks no published URL.
  // See research/15-docs-site-plan.md §7.4.
  locales: [
    {
      lang: 'en',
      label: 'English',
      title: SITE_TITLE,
      description: SITE_DESCRIPTION,
    },
  ],

  markdown: {
    checkDeadLinks: true,
    // Registered globally so 50-odd pages do not each open with an import
    // block. Each file's default export is bound to its capitalised filename.
    globalComponents: [
      path.join(__dirname, 'theme/components/Terminal.tsx'),
      path.join(__dirname, 'theme/components/Shot.tsx'),
      path.join(__dirname, 'theme/components/Keys.tsx'),
      path.join(__dirname, 'theme/components/Palette.tsx'),
      path.join(__dirname, 'theme/components/Cards.tsx'),
      path.join(__dirname, 'theme/components/Card.tsx'),
      path.join(__dirname, 'theme/components/Stats.tsx'),
      path.join(__dirname, 'theme/components/Stat.tsx'),
      path.join(__dirname, 'theme/components/Wordmark.tsx'),
    ],
  },

  themeConfig: {
    // Default to dark, keep the toggle. The tool is dark by default and so is
    // the audience — research/15-docs-site-plan.md §7.3.
    darkMode: 'dark',
    socialLinks: [
      {
        icon: 'github',
        mode: 'link',
        content: 'https://github.com/giancarlosisasi/labdash',
      },
    ],
    // No `footer` key here on purpose. Rspress routes themeConfig.footer.message
    // to <HomeFooter>, which renders only under `pageType: home`; this site's
    // homepage is `pageType: custom`, so it would reach no page. The footer is
    // theme/components/SiteFooter.tsx, mounted in the Layout's `bottom` slot.
    locales: [
      {
        lang: 'en',
        label: 'English',
        outlineTitle: 'On this page',
        prevPageText: 'Previous',
        nextPageText: 'Next',
        searchPlaceholderText: 'Search the docs',
        searchNoResultsText: 'No results for',
        searchSuggestedQueryText: 'Try another term.',
        lastUpdatedText: 'Last updated',
        overview: {
          filterNameText: 'Filter',
          filterPlaceholderText: 'Type a keyword',
          filterNoResultText: 'Nothing matches that.',
        },
      },
    ],
  },
});
