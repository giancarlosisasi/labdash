/**
 * The labdash wordmark.
 *
 * Generated once with figlet's `ANSI Shadow` font and committed as text — the
 * same asset the TUI ships, so the site and the splash screen cannot drift.
 * See research/17-design-system.md §11.3.
 *
 * Every glyph in this font is exactly 8 columns wide, so `lab` is columns 0–23
 * and `dash` is columns 24–55. That split is what carries the identity: `lab`
 * recedes into text.muted, `dash` takes the amber → coral gradient (§11.4).
 */

const SPLIT = 24;

const ANSI_SHADOW = [
  '██╗      █████╗ ██████╗ ██████╗  █████╗ ███████╗██╗  ██╗',
  '██║     ██╔══██╗██╔══██╗██╔══██╗██╔══██╗██╔════╝██║  ██║',
  '██║     ███████║██████╔╝██║  ██║███████║███████╗███████║',
  '██║     ██╔══██║██╔══██╗██║  ██║██╔══██║╚════██║██╔══██║',
  '███████╗██║  ██║██████╔╝██████╔╝██║  ██║███████║██║  ██║',
  '╚══════╝╚═╝  ╚═╝╚═════╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝',
];

/**
 * ANSI Shadow draws each letter as a solid face of `█` plus a drop shadow made
 * of `╗ ║ ╔ ═ ╚ ╝`. Painting both at one weight flattens the letterforms, so
 * shadow runs are dimmed a step.
 *
 * The gradient must stay on the *parent* element for the amber → coral sweep to
 * run continuously across all four letters of `dash`. Nested shadow runs
 * therefore override with a solid fill colour and paint on top of it, rather
 * than each starting a gradient of their own.
 */
function faces(segment: string) {
  return segment.split(/(█+)/).map((run, i) =>
    run.startsWith('█') ? (
      run
    ) : (
      // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length art
      <span className="ld-wordmark__shadow" key={i}>
        {run}
      </span>
    ),
  );
}

export interface WordmarkProps {
  /** Shown under the rule, letter-spaced. Omit for the bare mark. */
  tagline?: string;
}

export function Wordmark({
  tagline = 'every queue that matters',
}: WordmarkProps) {
  return (
    <div className="ld-wordmark">
      <pre className="ld-wordmark__art" aria-label="labdash" role="img">
        {ANSI_SHADOW.map((line, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: fixed-length art
          <span key={i}>
            <span className="ld-wordmark__lab">
              {faces(line.slice(0, SPLIT))}
            </span>
            <span className="ld-wordmark__dash">
              {faces(line.slice(SPLIT))}
            </span>
            {i < ANSI_SHADOW.length - 1 ? '\n' : ''}
          </span>
        ))}
      </pre>
      <div className="ld-wordmark__rule" />
      {tagline ? (
        <div className="ld-wordmark__tagline">{tagline}</div>
      ) : null}
    </div>
  );
}

/**
 * The single-line mark: one accent bar and the name, six cells. Used in the
 * navigation bar, where a six-row splash would be exactly the mistake §12
 * warns about.
 */
export function NavMark() {
  return (
    <a className="ld-navmark" href="/" aria-label="labdash — home">
      <span className="ld-navmark__bar" aria-hidden="true">
        ▍
      </span>
      <span className="ld-navmark__lab">lab</span>
      <span className="ld-navmark__dash">dash</span>
    </a>
  );
}

/**
 * The ASCII-only fallback (figlet `Standard`), shown on /about/design to make
 * the degradation path visible rather than claimed. `ACC-04`.
 */
export default Wordmark;

export const STANDARD_FIGLET = [
  ' _       _         _           _     ',
  '| | __ _| |__   __| | __ _ ___| |__  ',
  "| |/ _` | '_ \\ / _` |/ _` / __| '_ \\ ",
  '| | (_| | |_) | (_| | (_| \\__ \\ | | |',
  '|_|\\__,_|_.__/ \\__,_|\\__,_|___/_| |_|',
].join('\n');
