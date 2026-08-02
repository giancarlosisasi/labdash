import { ANALYTICS_ENABLED, clearConsent } from '../analytics';

/**
 * <SiteFooter> — rendered site-wide through the Layout's `bottom` slot.
 *
 * Rspress's own `themeConfig.footer.message` only reaches <HomeFooter>, which
 * only renders under `pageType: home`. This site's homepage is `pageType:
 * custom`, so that message appeared on no page at all, and the GitLab
 * disclaimer along with it. Hence a footer of our own, in a slot every page
 * gets.
 *
 * The author line carries `rel="me"`, which is what links this site to the
 * maintainer's own domain for anything that reads identity from the web.
 *
 * The Cookies control clears the stored choice so the banner asks again. It
 * renders only when analytics actually ships, so the site never offers a
 * control that does nothing. See ConsentBanner.tsx and docs/en/help/privacy.mdx.
 */
export function SiteFooter() {
  return (
    <footer className="ld-footer">
      <div className="ld-footer__inner">
        <nav className="ld-footer__links" aria-label="Site information">
          <a href="/help/privacy">Privacy</a>
          <a href="/help/security">Security</a>
          {ANALYTICS_ENABLED ? (
            <button type="button" onClick={clearConsent}>
              Cookies
            </button>
          ) : null}
          <a href="/about/contributing">Contributing</a>
          <a href="https://github.com/giancarlosisasi/labdash">GitHub</a>
        </nav>
        <div className="ld-footer__meta">
          <p className="ld-footer__author">
            Built by{' '}
            <a
              className="ld-footer__author-link"
              href="https://giancarlos-isasi.com/"
              rel="me author"
            >
              Giancarlos Isasi
            </a>
          </p>
          <p className="ld-footer__note">
            labdash is an independent open-source project. It is not affiliated
            with, endorsed by, or sponsored by GitLab Inc. GitLab is a trademark
            of GitLab Inc.
          </p>
        </div>
      </div>
    </footer>
  );
}

export default SiteFooter;
