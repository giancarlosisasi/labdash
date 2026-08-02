import type { ReactNode } from 'react';

export function Stat({ value, label }: { value: string; label: ReactNode }) {
  return (
    <div className="ld-stat">
      <span className="ld-stat__value">{value}</span>
      <span className="ld-stat__label">{label}</span>
    </div>
  );
}

export default Stat;
