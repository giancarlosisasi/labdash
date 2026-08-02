import { useEffect, useSyncExternalStore } from 'react';

import {
  ANALYTICS_ENABLED,
  readStoredConsent,
  resumeConsent,
  setConsent,
  subscribeConsent,
} from '../analytics';

/**
 * <ConsentBanner> — the analytics consent gate.
 *
 * Google Analytics writes an identifier to the visitor's device, which is not
 * strictly necessary for delivering a documentation site. Under ePrivacy
 * Article 5(3) that needs prior consent in the EEA and the UK, to the GDPR's
 * consent standard.
 *
 * Reject is the same size, the same row and the same click count as Accept.
 * A quieter reject button is the exact pattern regulators have penalised.
 *
 * The decision is read through useSyncExternalStore rather than useState, so
 * the banner closes in every open tab when the visitor decides in one of them.
 * The server snapshot is `null`, which renders nothing during SSG: the banner
 * belongs to the browser that holds the choice, not to the prerendered HTML.
 */

const SUB_NOOP = () => () => {};

export function ConsentBanner() {
  const decision = useSyncExternalStore(
    ANALYTICS_ENABLED ? subscribeConsent : SUB_NOOP,
    () => (ANALYTICS_ENABLED ? readStoredConsent() : 'denied'),
    () => null,
  );

  // A visitor who accepted on an earlier visit gets the tag without being
  // asked again. Nothing loads for anyone else.
  useEffect(() => {
    resumeConsent();
  }, []);

  if (!ANALYTICS_ENABLED || decision !== null) return null;

  return (
    <div
      className="ld-consent"
      role="region"
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
            onClick={() => setConsent('denied')}
          >
            Reject
          </button>
          <button
            type="button"
            className="ld-consent__btn ld-consent__btn--accept"
            onClick={() => setConsent('granted')}
          >
            Accept
          </button>
        </div>
      </div>
    </div>
  );
}

export default ConsentBanner;
