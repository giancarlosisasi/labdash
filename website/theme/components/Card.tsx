import type { ReactNode } from 'react';

export interface CardProps {
  title: string;
  href?: string;
  /** Small uppercase label above the title — a grouping, not a sentence. */
  eyebrow?: string;
  children?: ReactNode;
}

export function Card({ title, href, eyebrow, children }: CardProps) {
  const body = (
    <>
      {eyebrow ? (
        <div className="ld-card__eyebrow">
          <span>{eyebrow}</span>
        </div>
      ) : null}
      <h3 className="ld-card__title">{title}</h3>
      <div className="ld-card__body">{children}</div>
      {href ? <span className="ld-card__foot">Read →</span> : null}
    </>
  );

  return href ? (
    <a className="ld-card" href={href}>
      {body}
    </a>
  ) : (
    <div className="ld-card">{body}</div>
  );
}

export default Card;
