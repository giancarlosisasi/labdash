// Package layout turns a terminal size into every measurement a screen needs.
//
// Every value is a pure function of the size, the view and the section count.
// Nothing is adjusted incrementally and nothing is cached, which is what makes
// a resize round trip identical by construction rather than by luck.
//
// There is no layout setting, so the drop order in columns.go is the only
// answer to a narrow terminal and a user has no way to work around a bad one.
package layout

// MinWidth is the narrowest terminal that draws a dashboard. Below it the
// screen says what it needs instead of smearing.
const MinWidth = 56

// MinTabHeight is the shortest terminal that keeps the section tab bar. Below
// it the tabs are dropped before any data is.
const MinTabHeight = 12

// MinBottomPreviewHeight is the shortest terminal a bottom split is worth
// having in. Below it a preview would leave two rows of table.
const MinBottomPreviewHeight = 20

// MaxOverlayWidth caps an overlay however wide the terminal is.
const MaxOverlayWidth = 78

// A Breakpoint is one band of terminal width. Each is a real window somebody
// works in.
type Breakpoint int

const (
	// XS is too narrow to draw a table.
	XS Breakpoint = iota
	// S is a split pane: one pane, two-line rows.
	S
	// M is a single pane with the preview along the bottom.
	M
	// L is a split with the preview on the right.
	L
	// XL is the same split, at a narrower preview.
	XL
)

func (b Breakpoint) String() string {
	return [...]string{"xs", "s", "m", "l", "xl"}[b]
}

// A Preview is where the detail pane sits. There is no key and no setting for
// it: auto is the only position, and if auto is wrong the fix is auto.
type Preview int

const (
	NoPreview Preview = iota
	BottomPreview
	RightPreview
)

// A Layout is every measurement one frame needs.
type Layout struct {
	Width, Height int
	Breakpoint    Breakpoint

	// TooNarrow replaces the whole screen with a notice naming MinWidth.
	TooNarrow bool

	// The four chrome rows. SectionTabs is the only one that can be zero, and
	// it is the first thing dropped when rows are scarce.
	ContextBar, SectionTabs, TableHeader, Footer int

	// Frame is the rows spent on the border and the rules between regions.
	Frame int

	// Body is the rows left for data, preview included.
	Body int

	Preview       Preview
	PreviewWidth  int
	PreviewHeight int

	// TableWidth is the width of the row area, inside the frame and beside the
	// preview.
	TableWidth int
	// TableHeight is the rows the table has once the preview has taken its
	// share.
	TableHeight int

	// RowHeight is 1 everywhere except S, where a row needs two lines to carry
	// the three columns that never drop.
	RowHeight int

	OverlayWidth int
}

// Chrome is the rows spent on the context bar, the section tabs, the table
// header and the footer. The budget is four, and an addition is justified
// against it.
//
// The border and the rules between regions are the frame rather than chrome,
// and are counted by Frame.
func (l Layout) Chrome() int {
	return l.ContextBar + l.SectionTabs + l.TableHeader + l.Footer
}

// Compute is the whole of layout. Nothing else measures anything.
func Compute(width, height int) Layout {
	l := Layout{
		Width:        width,
		Height:       height,
		Breakpoint:   breakpointFor(width),
		OverlayWidth: overlayWidth(width),
		RowHeight:    1,
	}

	if width < MinWidth || height < 6 {
		l.TooNarrow = true
		return l
	}

	l.ContextBar, l.TableHeader, l.Footer = 1, 1, 1
	if height >= MinTabHeight {
		l.SectionTabs = 1
	}
	if l.Breakpoint == S {
		l.RowHeight = 2
	}

	// Two border rows, a rule under the context bar, a rule under the section
	// tabs when they are showing, and a rule above the footer.
	l.Frame = 4 + l.SectionTabs
	l.Body = height - l.Frame - l.Chrome()

	l.Preview = previewFor(l.Breakpoint, height)
	l.PreviewWidth = previewWidth(l.Preview, width, l.Breakpoint)

	content := width - 4 // one border and one space of padding on each side
	switch l.Preview {
	case RightPreview:
		l.TableWidth = content - l.PreviewWidth - 3 // the separator and its padding
		l.TableHeight = l.Body
		l.PreviewHeight = l.Body
	case BottomPreview:
		l.TableWidth = content
		l.PreviewHeight = l.Body * 40 / 100
		l.TableHeight = l.Body - l.PreviewHeight - 1 // the rule between them
	default:
		l.TableWidth = content
		l.TableHeight = l.Body
	}

	return l
}

func breakpointFor(width int) Breakpoint {
	switch {
	case width < MinWidth:
		return XS
	case width < 80:
		return S
	case width < 120:
		return M
	case width < 160:
		return L
	default:
		return XL
	}
}

// previewFor resolves auto. A right-hand pane on a 90-column terminal squeezes
// the table to uselessness, so auto is bottom below 120 columns and right at or
// above it.
func previewFor(b Breakpoint, height int) Preview {
	switch b {
	case L, XL:
		return RightPreview
	case M:
		if height < MinBottomPreviewHeight {
			return NoPreview
		}
		return BottomPreview
	default:
		return NoPreview
	}
}

func previewWidth(p Preview, width int, b Breakpoint) int {
	if p != RightPreview {
		return 0
	}
	if b == XL {
		return width * 34 / 100
	}
	return width * 38 / 100
}

func overlayWidth(width int) int {
	return min(MaxOverlayWidth, width-8)
}
