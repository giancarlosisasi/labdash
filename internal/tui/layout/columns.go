package layout

// A Column is one table column and the width it stops being worth its space at.
type Column struct {
	// Name is the identifier used in code, in golden file names and in the
	// documentation. One name for one thing.
	Name string
	// Header is the word above the column. It fits inside MinWidth, so the
	// narrowest columns still say what they are instead of truncating to an
	// ellipsis or to nothing.
	Header string
	// MinWidth is the cells it needs to say anything at all.
	MinWidth int
	// Grows gives a column the space left over. Exactly one column grows.
	Grows bool
	// DropsBelow is the terminal width it disappears at. Zero never drops.
	DropsBelow int
}

// Columns is the whole priority order, and it is permanent.
//
// Title, blockers and pipeline never drop, because those three are the product:
// what it is, why it is stuck, and whether it is green. Everything else is
// context and goes in this order.
//
// The widths are measured against the terminal rather than the table, so the
// width bands and this list are one specification instead of two.
func Columns() []Column {
	return []Column{
		{Name: "title", Header: "TITLE", MinWidth: 20, Grows: true},
		{Name: "blockers", Header: "BLOCKER", MinWidth: 10},
		{Name: "pipeline", Header: "CI", MinWidth: 2},
		{Name: "approvals", Header: "APPR", MinWidth: 4, DropsBelow: 60},
		{Name: "project", Header: "PROJECT", MinWidth: 10, DropsBelow: 68},
		{Name: "updated", Header: "AGE", MinWidth: 4, DropsBelow: 76},
		{Name: "author", Header: "AUTHOR", MinWidth: 8, DropsBelow: 90},
		{Name: "labels", Header: "LABELS", MinWidth: 12, DropsBelow: 110},
		{Name: "diff", Header: "DIFF", MinWidth: 9, DropsBelow: 124},
		{Name: "threads", Header: "THRDS", MinWidth: 5, DropsBelow: 136},
		{Name: "branch", Header: "BRANCH", MinWidth: 14, DropsBelow: 150},
	}
}

// ColumnsAt returns the columns a terminal of this width still shows.
func ColumnsAt(width int) []Column {
	var out []Column
	for _, c := range Columns() {
		if c.DropsBelow == 0 || width >= c.DropsBelow {
			out = append(out, c)
		}
	}
	return out
}

// Widths fits the surviving columns into the table area, giving the leftover to
// the one column that grows.
//
// A column is never given less than its minimum: if the sum does not fit, the
// growing column keeps its minimum and the caller truncates, which is what the
// ellipsis is for. Nothing wraps.
func Widths(l Layout, gap int) map[string]int {
	columns := ColumnsAt(l.Width)
	if len(columns) == 0 {
		return nil
	}

	widths := make(map[string]int, len(columns))
	fixed := gap * (len(columns) - 1)
	for _, c := range columns {
		widths[c.Name] = c.MinWidth
		if !c.Grows {
			fixed += c.MinWidth
		}
	}

	for _, c := range columns {
		if c.Grows {
			widths[c.Name] = max(c.MinWidth, l.TableWidth-fixed)
		}
	}
	return widths
}
