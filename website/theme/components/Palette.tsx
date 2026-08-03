/**
 * <Palette> — every semantic token, with its computed contrast.
 *
 * The numbers are not decoration. research/17-design-system.md §2.2 asserts
 * them on every commit (`ACC-02.T1`), and a theme that ships without a computed
 * ratio is the listed way inaccessible defaults get out of the door (§12).
 * Publishing them here is what makes that claim checkable by a reader.
 *
 * `labdash theme preview` prints the same numbers, computed in the reader's own
 * terminal.
 */

interface TokenRow {
  name: string;
  dark: string;
  light: string;
  /**
   * The worst contrast this token produces against any of the four surfaces —
   * bg.base, bg.surface, bg.overlay and bg.selected — not against bg.base
   * alone. The selected row is the surface that decides it, and a timestamp on
   * a selected row is the most-scanned cell in the product.
   *
   * Blank for the surfaces themselves and for the two decorative borders, which
   * carry no state and are exempt with a written reason.
   */
  ratioDark?: string;
  ratioLight?: string;
  use: string;
}

const SURFACES: TokenRow[] = [
  { name: 'bg.base', dark: '#0F1117', light: '#FBFBFD', use: 'The frame' },
  {
    name: 'bg.surface',
    dark: '#161922',
    light: '#F2F3F7',
    use: 'Header, footer, filter bar',
  },
  {
    name: 'bg.overlay',
    dark: '#1E222D',
    light: '#FFFFFF',
    use: 'Overlays and modals',
  },
  {
    name: 'bg.selected',
    dark: '#232838',
    light: '#E4E8F2',
    use: 'The selected row',
  },
  {
    name: 'border.faint',
    dark: '#262B38',
    light: '#E2E5EC',
    use: 'Row separators',
  },
  {
    name: 'border.default',
    dark: '#39404F',
    light: '#C3C9D6',
    use: 'Pane borders',
  },
  {
    name: 'border.focus',
    dark: '#E8A33D',
    light: '#8A4F00',
    ratioDark: '8.75:1',
    ratioLight: '6.35:1',
    use: 'The focused pane',
  },
];

const TEXT: TokenRow[] = [
  {
    name: 'text.primary',
    dark: '#E6E9EF',
    light: '#14161C',
    ratioDark: '12.06:1',
    ratioLight: '14.75:1',
    use: 'Titles and values',
  },
  {
    name: 'text.secondary',
    dark: '#A8B0C0',
    light: '#454C5C',
    ratioDark: '6.73:1',
    ratioLight: '7.01:1',
    use: 'Authors, projects, metadata',
  },
  {
    name: 'text.muted',
    dark: '#8A94A7',
    light: '#5F6778',
    ratioDark: '4.80:1',
    ratioLight: '4.63:1',
    use: 'Timestamps, hints, disabled',
  },
];

const MEANING: TokenRow[] = [
  {
    name: 'accent.primary',
    dark: '#E8A33D',
    light: '#8A4F00',
    ratioDark: '6.80:1',
    ratioLight: '5.35:1',
    use: 'Brand, focus, active tab',
  },
  {
    name: 'accent.secondary',
    dark: '#7AA2F7',
    light: '#2B58BC',
    ratioDark: '5.82:1',
    ratioLight: '5.31:1',
    use: 'Links and references',
  },
  {
    name: 'status.success',
    dark: '#7BD88F',
    light: '#106B36',
    ratioDark: '8.42:1',
    ratioLight: '5.39:1',
    use: 'passed, merged, approved',
  },
  {
    name: 'status.warning',
    dark: '#E8C55D',
    light: '#7A5600',
    ratioDark: '8.78:1',
    ratioLight: '5.42:1',
    use: 'manual, stale, allow-failure',
  },
  {
    name: 'status.error',
    dark: '#F07178',
    light: '#B32E38',
    ratioDark: '5.12:1',
    ratioLight: '5.09:1',
    use: 'failed, conflicts, blocked',
  },
  {
    name: 'status.running',
    dark: '#5FD7E0',
    light: '#0C6169',
    ratioDark: '8.58:1',
    ratioLight: '5.85:1',
    use: 'running, in progress',
  },
  {
    name: 'status.pending',
    dark: '#C4A7F5',
    light: '#603AAB',
    ratioDark: '7.12:1',
    ratioLight: '6.36:1',
    use: 'pending, queued, scheduled',
  },
  {
    name: 'status.neutral',
    dark: '#8A94A7',
    light: '#5F6778',
    ratioDark: '4.80:1',
    ratioLight: '4.63:1',
    use: 'skipped, canceled, created',
  },
];

const GROUPS: Record<string, TokenRow[]> = {
  surfaces: SURFACES,
  text: TEXT,
  meaning: MEANING,
};

export interface PaletteProps {
  group?: 'surfaces' | 'text' | 'meaning';
  theme?: 'dark' | 'light';
}

export function Palette({ group = 'meaning', theme = 'dark' }: PaletteProps) {
  const rows = GROUPS[group] ?? MEANING;

  return (
    <div className="ld-swatches">
      {rows.map((row) => {
        const hex = theme === 'dark' ? row.dark : row.light;
        const ratio = theme === 'dark' ? row.ratioDark : row.ratioLight;
        return (
          <div className="ld-swatch" key={row.name}>
            <span
              className="ld-swatch__chip"
              style={{ background: hex }}
              aria-hidden="true"
            />
            <span className="ld-swatch__meta">
              <span className="ld-swatch__name">{row.name}</span>
              <span className="ld-swatch__val">
                {hex}
                {ratio ? ` · ${ratio}` : ''}
              </span>
            </span>
          </div>
        );
      })}
    </div>
  );
}

export default Palette;
