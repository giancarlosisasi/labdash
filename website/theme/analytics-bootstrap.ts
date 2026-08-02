import { CONSENT_STORAGE_KEY } from './analytics';

/**
 * The inline <head> script, injected through `head` in rspress.config.ts.
 *
 * It runs synchronously, before anything else, and does one job: put the
 * Consent Mode v2 default state on `dataLayer` so it is already correct when
 * gtag.js arrives later. Everything is denied; `analytics_storage` follows the
 * visitor's stored choice, so a returning visitor who accepted does not lose
 * their first pageview to a race.
 *
 * The ad signals are denied and stay denied. This site runs no advertising, and
 * writing them out explicitly means a future gtag change cannot quietly default
 * one of them to granted.
 *
 * `wait_for_update` gives the banner half a second to answer on a first visit
 * before gtag decides what to do, which is what stops a flicker of the wrong
 * state.
 *
 * Kept as a string in its own module so the client bundle for ConsentBanner
 * does not carry it.
 */
export const consentBootstrapScript = `(function(){try{window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}window.gtag=gtag;var c=localStorage.getItem('${CONSENT_STORAGE_KEY}');gtag('consent','default',{ad_storage:'denied',ad_user_data:'denied',ad_personalization:'denied',analytics_storage:c==='granted'?'granted':'denied',wait_for_update:500});}catch(e){}})();`;
