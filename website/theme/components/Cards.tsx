import type { ReactNode } from 'react';

/** A responsive grid. Nothing decorative; it exists to stop 17 flat links. */
export function Cards({
  children,
  tight,
}: {
  children: ReactNode;
  tight?: boolean;
}) {
  return (
    <div className={`ld-cards${tight ? ' ld-cards--tight' : ''}`}>
      {children}
    </div>
  );
}

export default Cards;
