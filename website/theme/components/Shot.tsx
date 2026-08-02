import type { ReactNode } from 'react';

/**
 * <Shot> — the media slot on a content page.
 *
 * It renders one of three things, in this order of preference:
 *
 *   <Shot src="/shots/home.png" …/>        an image
 *   <Shot video="/shots/browse.mp4" …/>    a looping, muted recording
 *   <Shot …/>                              the frame, with `alt` as prose
 *
 * The third form is not a "missing asset" marker. `alt` is written as a
 * description of the screen, so a page with no capture yet still tells the
 * reader what they would be looking at, and a reader who cannot see the capture
 * gets the same information either way. Dropping a file in later is a one-line
 * edit and nothing around it moves, because the frame keeps its aspect ratio.
 *
 * `alt` is therefore required on every call.
 */

export interface ShotProps {
  /** Stable slug for the screen. Shown in the frame's chrome bar. */
  id: string;
  /**
   * What the screen shows. Required. Doubles as the alt text and as the
   * specification for whoever captures it.
   */
  alt: string;
  /** Image source, e.g. `/shots/home.png`. */
  src?: string;
  /** Recording source, e.g. `/shots/browse.mp4`. Takes a poster from `src`. */
  video?: string;
  /** Prose under the frame. */
  caption?: ReactNode;
  /** Capture width in columns. 120x32 is the site-wide standard. */
  cols?: number;
  /** Capture height in rows. */
  rows?: number;
}

export function Shot({
  id,
  alt,
  src,
  video,
  caption,
  cols = 120,
  rows = 32,
}: ShotProps) {
  return (
    <figure
      className="ld-term ld-shot"
      style={{ ['--ld-shot-aspect' as string]: `${cols} / ${rows}` }}
    >
      <div className="ld-term__chrome">
        <span className="ld-term__bar" aria-hidden="true">
          ▍
        </span>
        <span className="ld-term__title">{id}</span>
        <span className="ld-term__dims" aria-hidden="true">
          {cols}×{rows}
        </span>
      </div>
      <div className="ld-shot__body">
        {video ? (
          // biome-ignore lint/a11y/useMediaCaption: aria-label carries the description
          <video
            className="ld-shot__media"
            src={video}
            poster={src}
            aria-label={alt}
            autoPlay
            loop
            muted
            playsInline
          />
        ) : src ? (
          <img className="ld-shot__media" src={src} alt={alt} />
        ) : (
          <div className="ld-shot__frame" role="img" aria-label={alt}>
            <span className="ld-shot__id">{id}</span>
            <span className="ld-shot__note">{alt}</span>
          </div>
        )}
      </div>
      {caption ? (
        <figcaption className="ld-term__caption">{caption}</figcaption>
      ) : null}
    </figure>
  );
}

export default Shot;
