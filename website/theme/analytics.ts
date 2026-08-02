/**
 * Google Analytics 4 + Consent Mode v2, and the consent helpers around it.
 *
 * Two rules shape this file.
 *
 * 1. gtag.js is requested only after the visitor accepts. Consent Mode's
 *    "advanced" pattern loads the tag immediately and sends cookieless pings
 *    while storage is denied, which is defensible but still puts the visitor's
 *    IP in front of Google before they agreed. A documentation site gains
 *    nothing from those pings, so we take the stricter path: nothing is
 *    requested until Accept.
 *
 * 2. The Consent Mode defaults are still written synchronously in <head>, by
 *    `analytics-bootstrap.ts`, before anything else runs. That way a returning
 *    visitor who already accepted has the correct state in `dataLayer` when the
 *    tag arrives, and their first pageview is not lost.
 *
 * The measurement ID is a public value: it ships in the page and travels in
 * every request. It is a constant here, not a secret.
 */

/**
 * Replace with the GA4 measurement ID for this site.
 *
 * While this is the placeholder, analytics is off end to end: no banner, no
 * cookies, no requests. That keeps the site honest before the property exists,
 * and switching it on is this one line.
 */
export const GA_MEASUREMENT_ID = 'G-XXXXXXXXXX';

const PLACEHOLDER_ID = 'G-XXXXXXXXXX';

/** localStorage key holding the visitor's choice. */
export const CONSENT_STORAGE_KEY = 'labdash.ga-consent';

export type ConsentChoice = 'granted' | 'denied';

/** Fired on a same-tab change so the banner can re-read without a reload. */
const CONSENT_CHANGE_EVENT = 'labdash:consentchange';

declare global {
  interface Window {
    dataLayer?: unknown[];
    gtag?: (...args: unknown[]) => void;
  }
}

/**
 * Whether this build ships analytics at all.
 *
 * Off until a real measurement ID is set, and off outside production, so local
 * development and preview builds never reach the GA property. Both conditions
 * are checked at module scope, so a disabled build tree-shakes the loader away
 * entirely rather than shipping dead code.
 */
export const ANALYTICS_ENABLED =
  GA_MEASUREMENT_ID !== '' &&
  GA_MEASUREMENT_ID !== PLACEHOLDER_ID &&
  process.env.NODE_ENV === 'production';

/** The stored choice, or null when the visitor has not chosen yet. */
export function readStoredConsent(): ConsentChoice | null {
  if (typeof window === 'undefined') return null;
  try {
    const v = window.localStorage.getItem(CONSENT_STORAGE_KEY);
    return v === 'granted' || v === 'denied' ? v : null;
  } catch {
    // Private mode, or storage disabled. Undecided, and nothing is tracked.
    return null;
  }
}

/** Injects gtag.js. Called only from `setConsent` and only on 'granted'. */
function loadAnalytics(): void {
  if (!ANALYTICS_ENABLED) return;
  if (document.getElementById('ga-tag')) return;

  window.dataLayer = window.dataLayer || [];
  const gtag: (...args: unknown[]) => void = (...args) => {
    window.dataLayer?.push(args);
  };
  window.gtag = window.gtag ?? gtag;

  const s = document.createElement('script');
  s.id = 'ga-tag';
  s.async = true;
  s.src = `https://www.googletagmanager.com/gtag/js?id=${GA_MEASUREMENT_ID}`;
  document.head.appendChild(s);

  window.gtag('js', new Date());
  window.gtag('config', GA_MEASUREMENT_ID, { anonymize_ip: true });
}

/**
 * Persists the choice, updates Consent Mode live so gtag reacts without a
 * reload, and loads the tag the first time consent is granted.
 */
export function setConsent(choice: ConsentChoice): void {
  try {
    window.localStorage.setItem(CONSENT_STORAGE_KEY, choice);
  } catch {
    // Cannot remember it for next time. The session still honours the choice.
  }

  window.gtag?.('consent', 'update', { analytics_storage: choice });
  if (choice === 'granted') loadAnalytics();

  window.dispatchEvent(new Event(CONSENT_CHANGE_EVENT));
}

/**
 * Forgets the choice so the banner asks again, and denies storage in the
 * meantime. This is what the footer's Cookies control calls: withdrawing
 * consent has to be as easy as giving it, so it cannot live only in the
 * first-visit banner.
 */
export function clearConsent(): void {
  try {
    window.localStorage.removeItem(CONSENT_STORAGE_KEY);
  } catch {
    // Nothing stored to forget.
  }
  window.gtag?.('consent', 'update', { analytics_storage: 'denied' });
  window.dispatchEvent(new Event(CONSENT_CHANGE_EVENT));
}

/** Same-tab changes via the custom event, cross-tab via `storage`. */
export function subscribeConsent(onChange: () => void): () => void {
  window.addEventListener(CONSENT_CHANGE_EVENT, onChange);
  window.addEventListener('storage', onChange);
  return () => {
    window.removeEventListener(CONSENT_CHANGE_EVENT, onChange);
    window.removeEventListener('storage', onChange);
  };
}

/** Loads the tag on boot for a visitor who accepted on an earlier visit. */
export function resumeConsent(): void {
  if (ANALYTICS_ENABLED && readStoredConsent() === 'granted') loadAnalytics();
}
