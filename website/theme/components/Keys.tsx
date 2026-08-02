import type { ReactNode } from 'react';

/**
 * <Keys> — renders a binding as key caps rather than as code.
 *
 * `<Keys>g g</Keys>` is two caps; `<Keys>Ctrl+K</Keys>` is one. Splitting on
 * spaces and never on `+` keeps chords intact, which matters because labdash
 * has no timed key sequences — a chord is Ctrl-prefixed and instantaneous
 * (research/17-design-system.md §6.3 rule 6).
 */
export interface KeysProps {
  children: ReactNode;
}

export function Keys({ children }: KeysProps) {
  const caps = String(children ?? '')
    .split(/\s+/)
    .filter(Boolean);

  return (
    <span className="ld-keys">
      {caps.map((cap, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: stable literal input
        <kbd key={`${cap}-${i}`} className="ld-key">
          {cap}
        </kbd>
      ))}
    </span>
  );
}

export default Keys;
