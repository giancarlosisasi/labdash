/**
 * <SiteFooter> — rendered site-wide through the Layout's `bottom` slot.
 *
 * Rspress's own `themeConfig.footer.message` only reaches <HomeFooter>, which
 * only renders under `pageType: home`. This site's homepage is `pageType:
 * custom`, so that message appeared on no page at all, and the GitLab
 * disclaimer along with it. Hence a footer of our own, in a slot that every
 * page gets.
 *
 * The Cookies control reopens the analytics choice. Consent must be as easy to
 * withdraw as to give, which means it cannot live only in the first-visit
 * banner. See ConsentBanner.tsx and docs/en/help/privacy.mdx.
 */
import { ANALYTICS_ENABLED, reopenConsent } from './ConsentBanner';
export function SiteFooter() {
  return (
    <footer className="ld-footer">
      <div className="ld-footer__inner">
        <nav className="ld-footer__links" aria-label="Site information">
          <a href="/help/privacy">Privacy</a>
          <a href="/help/security">Security</a>
          {/* Withdrawing consent has to be as easy as giving it, so the choice
              is reachable from every page rather than only on first visit. */}
          {ANALYTICS_ENABLED ? (
            <button type="button" onClick={reopenConsent}>
              Cookies
            </button>
          ) : null}
          <a href="/about/contributing">Contributing</a>
          <a href="https://github.com/giancarlosisasi/labdash">GitHub</a>
        </nav>
        <p className="ld-footer__note">
          labdash is an independent open-source project. It is not affiliated
          with, endorsed by, or sponsored by GitLab Inc. GitLab is a trademark
          of GitLab Inc.
        </p>
      </div>
    </footer>
  );
}

export default SiteFooter;
