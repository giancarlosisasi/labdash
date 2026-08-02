import type { ReactNode } from 'react';

/**
 * <Terminal> — the picture on every feature page.
 *
 * The tool has no released build yet, so there is nothing honest to record.
 * What does exist is the screen inventory in research/16-screens-and-flows.md,
 * drawn cell by cell at 120x32. This component renders those drawings with the
 * *real* palette from research/17-design-system.md, which makes them a truthful
 * picture of the design rather than a decorative mock.
 *
 * When recordings exist, this component keeps its place: a `.tape` render drops
 * into the same frame and every page around it stays as written.
 *
 * The highlighter below is deliberately vocabulary-driven, not syntax-driven.
 * It colours the *status vocabulary* — glyph, colour and word travelling
 * together (§3) — so what you read here is the same triple the terminal prints.
 */

interface Token {
  name: string;
  re: string;
  cls: string;
}

// Order is precedence: at a given position, the first pattern that matches wins.
const TOKENS: Token[] = [
  // The active view name in the context bar: `▍MERGE REQUESTS`.
  { name: 'activeView', re: '▍[A-Z][A-Z0-9 ]*[A-Z]', cls: 't-acc' },

  // Frame and rules.
  { name: 'box', re: '[╭╮╰╯├┤┬┴┼─│┄━┃]+', cls: 't-border' },
  { name: 'skeleton', re: '[░▒▓]+', cls: 't-faint' },

  // The rate-limit meter.
  { name: 'meterOn', re: '▰+', cls: 't-ok' },
  { name: 'meterOff', re: '▱+', cls: 't-faint' },

  // Motion and selection.
  { name: 'spinner', re: '[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]', cls: 't-acc' },
  { name: 'marker', re: '[→▍▣✱❯]', cls: 't-acc' },

  // The status glyph set (§3.1). Never colour alone — the word is beside it.
  { name: 'glyphOk', re: '[✔✓󰄬]', cls: 't-ok' },
  { name: 'glyphErr', re: '[✖✗]', cls: 't-err' },
  { name: 'glyphRun', re: '●', cls: 't-run' },
  { name: 'glyphNeu', re: '⊘', cls: 't-neu' },
  { name: 'glyphWarn', re: '[⏸⚠]', cls: 't-warn' },
  { name: 'glyphMut', re: '[▸▾▴▲⋯‹›⟳⌗·◂⏎↑↓⇞⇟⇥⌃]', cls: 't-mut' },

  // References and identities.
  { name: 'mrRef', re: '![0-9]+', cls: 't-link' },
  { name: 'pipeRef', re: '#[0-9]+', cls: 't-link' },
  { name: 'user', re: '@[A-Za-z0-9_.-]+', cls: 't-link' },
  { name: 'train', re: 'train [0-9]+/[0-9]+', cls: 't-link' },

  // Diff size (§2.2 diff.added / diff.removed).
  { name: 'diffAdd', re: '\\+[0-9]+', cls: 't-add' },
  { name: 'diffDel', re: '−[0-9]+', cls: 't-del' },

  // Test-runner words inside a job trace.
  { name: 'logPass', re: '\\bPASS\\b', cls: 't-ok' },
  { name: 'logFail', re: '\\bFAIL\\b', cls: 't-err' },

  // The status words themselves.
  { name: 'wOk', re: '\\b(?:passed|merged|approved|ready)\\b', cls: 't-ok' },
  {
    name: 'wErr',
    re: '\\b(?:failed|conflicts|blocked|cannot merge|changes requested)\\b',
    cls: 't-err',
  },
  { name: 'wRun', re: '\\brunning\\b', cls: 't-run' },
  {
    name: 'wPend',
    re: '\\b(?:pending|queued|scheduled|waiting|preparing)\\b',
    cls: 't-pend',
  },
  {
    name: 'wWarn',
    re: '\\b(?:manual|stale|draft|unresolved|needs(?: [0-9]+| rebase| approval)?)\\b',
    cls: 't-warn',
  },
  {
    name: 'wNeu',
    re: '\\b(?:skipped|cancell?ed|cancelling|canceling|created|checking|closed|no pipeline)\\b',
    cls: 't-neu',
  },

  // Column headers and the view name. UPPERCASE is only ever chrome (§4).
  { name: 'header', re: '\\b[A-Z]{3,}(?: [A-Z]{3,})*\\b', cls: 't-hdr' },
];

const PATTERN = new RegExp(
  TOKENS.map((t) => `(?<${t.name}>${t.re})`).join('|'),
  'gu',
);

const CLASS_OF: Record<string, string> = Object.fromEntries(
  TOKENS.map((t) => [t.name, t.cls]),
);

function highlightLine(line: string, lineIndex: number): ReactNode[] {
  const out: ReactNode[] = [];
  let cursor = 0;
  let n = 0;

  for (const match of line.matchAll(PATTERN)) {
    const at = match.index ?? 0;
    if (at > cursor) out.push(line.slice(cursor, at));

    const groups = match.groups ?? {};
    const name = Object.keys(groups).find((k) => groups[k] !== undefined);
    out.push(
      <span key={`${lineIndex}-${n++}`} className={name ? CLASS_OF[name] : ''}>
        {match[0]}
      </span>,
    );
    cursor = at + match[0].length;
  }

  if (cursor < line.length) out.push(line.slice(cursor));
  return out;
}

/** Trim blank edges and strip the common indent an MDX literal picks up. */
function dedent(source: string): string[] {
  const lines = source.replace(/\t/g, '    ').split('\n');
  while (lines.length && lines[0].trim() === '') lines.shift();
  while (lines.length && lines[lines.length - 1].trim() === '') lines.pop();

  const indents = lines
    .filter((l) => l.trim() !== '')
    .map((l) => l.length - l.trimStart().length);
  const strip = indents.length ? Math.min(...indents) : 0;

  return lines.map((l) => l.slice(strip));
}

export interface TerminalProps {
  /** The ASCII screen, as a template literal child. */
  children: string;
  /** Shown in the chrome bar. Name the screen, not the product. */
  title?: string;
  /** Overrides the auto-measured column count used to size the type. */
  cols?: number;
  /** Prose under the frame. Required when the screen needs explaining. */
  caption?: ReactNode;
  /** Draws the frame in border.focus, for "this pane has focus" examples. */
  focus?: boolean;
  /**
   * Describes what the screen shows, for anyone who cannot see it. A screen
   * without one is the same bug as a recording without alt text (§5.3).
   */
  alt?: string;
}

export function Terminal({
  children,
  title,
  cols,
  caption,
  focus,
  alt,
}: TerminalProps) {
  const lines = dedent(String(children ?? ''));
  const measured = lines.reduce(
    (max, line) => Math.max(max, [...line].length),
    0,
  );
  const columns = cols ?? Math.max(measured, 40);
  const rows = lines.length;

  return (
    <figure
      className={`ld-term${focus ? ' ld-term--focus' : ''}`}
      style={{ ['--ld-cols' as string]: String(columns) }}
    >
      <div className="ld-term__chrome">
        <span className="ld-term__bar" aria-hidden="true">
          ▍
        </span>
        <span className="ld-term__title">{title ?? 'labdash'}</span>
        <span className="ld-term__dims" aria-hidden="true">
          {columns}×{rows}
        </span>
      </div>
      <div className="ld-term__body">
        <pre className="ld-term__pre" role="img" aria-label={alt ?? title}>
          {lines.map((line, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static art
            <span key={i}>
              {highlightLine(line, i)}
              {i < lines.length - 1 ? '\n' : ''}
            </span>
          ))}
        </pre>
      </div>
      {caption ? (
        <figcaption className="ld-term__caption">{caption}</figcaption>
      ) : null}
    </figure>
  );
}

export default Terminal;
