import { useEffect, useState } from 'react';

/**
 * <ConsentBanner> — the analytics consent gate.
 *
 * Google Analytics writes an identifier to the visitor's device. That is not
 * strictly necessary for delivering a documentation site, so under ePrivacy
 * Article 5(3) it needs prior consent in the EEA and the UK, to the GDPR's
 * consent standard.
 *
 * The load-bearing detail is that the tag is injected by loadAnalytics() and
 * by nothing else. It is never in the document head, and it never runs on
 * boot unless a stored decision already says 'granted'. Injecting first and
 * suppressing afterwards fails, because the storage itself is the thing that
 * needs consent.
 *
 * Reject is the same size, the same row and the same click count as Accept.
 * A quieter reject button is the exact pattern regulators have penalised.
 *
 * Storing the decision is itself exempt: it is a preference the visitor set,
 * and remembering it is what honours the choice.
 *
 * See docs/en/help/privacy.mdx, which must keep saying what this file does.
 */

/** Set to the GA4 measurement ID. While empty, no banner and no tag. */
const MEASUREMENT_ID = '';

const KEY = 'labdash.analytics-consent';
const REOPEN_EVENT = 'labdash:reopen-consent';

type Decision = 'granted' | 'denied';

function readDecision(): Decision | null {
  try {
    const v = localStorage.getItem(KEY);
    return v === 'granted' || v === 'denied' ? v : null;
  } catch {
    // Private mode, or storage disabled. Treat as undecided and do not track.
    return null;
  }
}

function loadAnalytics() {
  if (!MEASUREMENT_ID || document.getElementById('ga-tag')) return;

  const w = window as unknown as {
    dataLayer?: unknown[];
    gtag?: (...args: unknown[]) => void;
  };
  w.dataLayer = w.dataLayer || [];
  const gtag = (...args: unknown[]) => {
    w.dataLayer?.push(args);
  };
  w.gtag = gtag;

  gtag('consent', 'default', { analytics_storage: 'denied' });
  gtag('consent', 'update', { analytics_storage: 'granted' });

  const s = document.createElement('script');
  s.id = 'ga-tag';
  s.async = true;
  s.src = `https://www.googletagmanager.com/gtag/js?id=${MEASUREMENT_ID}`;
  document.head.appendChild(s);

  gtag('js', new Date());
  gtag('config', MEASUREMENT_ID, { anonymize_ip: true });
}

/**
 * Whether this build ships analytics at all. The footer's Cookies control
 * hides when it is false, so the site never offers a choice that does nothing.
 */
export const ANALYTICS_ENABLED = MEASUREMENT_ID !== '';

/** Footer link handler: lets a visitor change a decision already made. */
export function reopenConsent() {
  window.dispatchEvent(new CustomEvent(REOPEN_EVENT));
}

export function ConsentBanner() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!MEASUREMENT_ID) return;

    if (readDecision() === 'granted') loadAnalytics();
    else if (readDecision() === null) setOpen(true);

    const reopen = () => setOpen(true);
    window.addEventListener(REOPEN_EVENT, reopen);
    return () => window.removeEventListener(REOPEN_EVENT, reopen);
  }, []);

  if (!open) return null;

  const decide = (decision: Decision) => {
    try {
      localStorage.setItem(KEY, decision);
    } catch {
      // Nothing to do. Without storage the choice cannot be remembered, and
      // the honest fallback is to not track for this session either.
    }
    if (decision === 'granted') loadAnalytics();
    setOpen(false);
  };

  return (
    <div
      className="ld-consent"
      role="dialog"
      aria-modal="false"
      aria-label="Analytics cookies"
    >
      <div className="ld-consent__inner">
        <p className="ld-consent__text">
          This site can use Google Analytics to count page views. It stores an
          identifier in your browser and sends your IP address, browser and the
          pages you read to Google. Nothing loads unless you accept.{' '}
          <a href="/help/privacy">What is collected</a>.
        </p>
        <div className="ld-consent__actions">
          <button
            type="button"
            className="ld-consent__btn"
            onClick={() => decide('denied')}
          >
            Reject
          </button>
          <button
            type="button"
            className="ld-consent__btn ld-consent__btn--accept"
            onClick={() => decide('granted')}
          >
            Accept
          </button>
        </div>
      </div>
    </div>
  );
}

export default ConsentBanner;
