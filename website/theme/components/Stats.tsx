import type { ReactNode } from 'react';

/** Numbers, not adjectives (§5.1). Every value inside is measured, not claimed. */
export function Stats({ children }: { children: ReactNode }) {
  return <div className="ld-stats">{children}</div>;
}

export default Stats;
