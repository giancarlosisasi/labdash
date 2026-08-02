import * as path from 'node:path';
import { defineConfig, type UserConfig } from '@rspress/core';

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

/**
 * Typed against `UserConfig['head']` rather than inferred. A bare array literal
 * widens `['meta', {...}]` to `(string | object)[]`, which does not match the
 * `[string, Record<string, string>]` tuple the option expects, and the failure
 * surfaces as an unrelated "'root' does not exist" overload error on the whole
 * config object.
 */
const head: UserConfig['head'] = [
  // Runs before anything else so Consent Mode's default state is on the
  // dataLayer when gtag.js arrives. It stores nothing and requests nothing.
  // See theme/analytics-bootstrap.ts.
  `<script>${consentBootstrapScript}</script>`,

  ['meta', { name: 'author', content: AUTHOR_NAME }],
  ['link', { rel: 'author', href: AUTHOR_URL }],
  ['link', { rel: 'me', href: AUTHOR_URL }],

  `<script type="application/ld+json">${JSON.stringify(STRUCTURED_DATA)}</script>`,
];

const cfg: UserConfig = {
  root: path.join(__dirname, 'docs'),
  lang: 'en',
  title: SITE_TITLE,
  description: SITE_DESCRIPTION,
  icon: '/favicon.svg',
  head,

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

  // Every UI string the default theme shows, overridden for English only.
  //
  // These used to sit on `themeConfig.locales[].outlineTitle` and friends, which
  // Rspress 2 no longer accepts: LocaleConfig now holds lang, label, title,
  // description, nav and sidebar, and nothing else. The strings moved here.
  //
  // The function form takes the shipped defaults and returns the merged set, so
  // overriding `en` leaves every other language intact. The object form would
  // demand a `zh` value for each key alongside the `en` one.
  i18nSource: (defaults) => ({
    ...defaults,
    outlineTitle: { ...defaults.outlineTitle, en: 'On this page' },
    prevPageText: { ...defaults.prevPageText, en: 'Previous' },
    nextPageText: { ...defaults.nextPageText, en: 'Next' },
    lastUpdatedText: { ...defaults.lastUpdatedText, en: 'Last updated' },
    searchPlaceholderText: {
      ...defaults.searchPlaceholderText,
      en: 'Search the docs',
    },
    searchNoResultsText: {
      ...defaults.searchNoResultsText,
      en: 'No results for',
    },
    searchSuggestedQueryText: {
      ...defaults.searchSuggestedQueryText,
      en: 'Try another term.',
    },
    'overview.filterNameText': {
      ...defaults['overview.filterNameText'],
      en: 'Filter',
    },
    'overview.filterPlaceholderText': {
      ...defaults['overview.filterPlaceholderText'],
      en: 'Type a keyword',
    },
    'overview.filterNoResultText': {
      ...defaults['overview.filterNoResultText'],
      en: 'Nothing matches that.',
    },
  }),

  markdown: {
    // `markdown.checkDeadLinks` was silently ignored: in Rspress 2 the option
    // lives under `link`, and it already defaults to true. Anchor checking does
    // not, so it is switched on here. It catches a link to a heading that a
    // retitling moved, which is the failure a page rename actually causes.
    link: {
      checkDeadLinks: true,
      checkAnchors: true,
    },
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
    // UI strings live in `i18nSource` above, not here.
    locales: [{ lang: 'en', label: 'English' }],
  },
};

export default defineConfig(cfg);
