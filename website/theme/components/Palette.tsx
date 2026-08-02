/**
 * <Palette> — every semantic token, with its computed contrast against the
 * theme's own background.
 *
 * The numbers are not decoration. research/17-design-system.md §2.2 asserts
 * them on every commit (`ACC-02.T1`), and a theme that ships without a computed
 * ratio is the listed way inaccessible defaults get out of the door (§12).
 * Publishing them here is what makes that claim checkable by a reader.
 */

interface TokenRow {
  name: string;
  dark: string;
  light: string;
  /** Contrast against bg.base, dark theme. Blank for surfaces and borders. */
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
    ratioDark: '12.06:1',
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
    light: '#A85E00',
    use: 'The focused pane',
  },
];

const TEXT: TokenRow[] = [
  {
    name: 'text.primary',
    dark: '#E6E9EF',
    light: '#14161C',
    ratioDark: '15.35:1',
    ratioLight: '16.9:1',
    use: 'Titles and values',
  },
  {
    name: 'text.secondary',
    dark: '#A8B0C0',
    light: '#454C5C',
    ratioDark: '8.56:1',
    ratioLight: '8.9:1',
    use: 'Authors, projects, metadata',
  },
  {
    name: 'text.muted',
    dark: '#7C8699',
    light: '#656D7E',
    ratioDark: '5.09:1',
    ratioLight: '5.03:1',
    use: 'Timestamps, hints, disabled',
  },
];

const MEANING: TokenRow[] = [
  {
    name: 'accent.primary',
    dark: '#E8A33D',
    light: '#A85E00',
    ratioDark: '8.65:1',
    ratioLight: '4.77:1',
    use: 'Brand, focus, active tab',
  },
  {
    name: 'accent.secondary',
    dark: '#7AA2F7',
    light: '#2E5FCC',
    ratioDark: '7.41:1',
    ratioLight: '6.3:1',
    use: 'Links and references',
  },
  {
    name: 'status.success',
    dark: '#7BD88F',
    light: '#147A3D',
    ratioDark: '10.72:1',
    ratioLight: '5.25:1',
    use: 'passed, merged, approved',
  },
  {
    name: 'status.warning',
    dark: '#E8C55D',
    light: '#8A6100',
    ratioDark: '11.18:1',
    ratioLight: '5.1:1',
    use: 'manual, stale, allow-failure',
  },
  {
    name: 'status.error',
    dark: '#F07178',
    light: '#C0323C',
    ratioDark: '6.52:1',
    ratioLight: '5.41:1',
    use: 'failed, conflicts, blocked',
  },
  {
    name: 'status.running',
    dark: '#5FD7E0',
    light: '#0E6E77',
    ratioDark: '10.91:1',
    ratioLight: '5.6:1',
    use: 'running, in progress',
  },
  {
    name: 'status.pending',
    dark: '#C4A7F5',
    light: '#6B3FBF',
    ratioDark: '9.06:1',
    ratioLight: '6.4:1',
    use: 'pending, queued, scheduled',
  },
  {
    name: 'status.neutral',
    dark: '#7C8699',
    light: '#656D7E',
    ratioDark: '5.09:1',
    ratioLight: '5.03:1',
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
