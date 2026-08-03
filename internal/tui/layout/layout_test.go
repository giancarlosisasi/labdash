package layout_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
)

func TestMain(m *testing.M) { harness.Main(m) }

// SHL-14.T1, at the level the guarantee actually lives. Prevents resize
// corruption: every measurement comes from the size alone, so a round trip
// cannot land anywhere but where it started.
func TestSHL14_T1_AResizeRoundTripIsIdentical(t *testing.T) {
	t.Parallel()

	start := layout.Compute(200, 50)

	for _, width := range []int{190, 160, 140, 120, 100, 90, 100, 120, 140, 160, 190} {
		layout.Compute(width, 50)
	}

	require.Equal(t, start, layout.Compute(200, 50))
}

// The same guarantee stated as the property that makes it true, so a future
// cached intermediate fails here rather than on a user's screen.
func TestSHL14_LayoutDependsOnNothingButTheSize(t *testing.T) {
	t.Parallel()

	for width := layout.MinWidth - 10; width <= 200; width++ {
		for _, height := range []int{8, 12, 20, 32, 50} {
			require.Equal(t, layout.Compute(width, height), layout.Compute(width, height),
				"%dx%d computed two different layouts", width, height)
		}
	}
}

// SHL-15.T1. Prevents unusable output on a split pane, which is where most of
// these terminals are.
func TestSHL15_T1_SeventyColumnsHidesThePreviewAndDropsColumns(t *testing.T) {
	t.Parallel()

	l := layout.Compute(70, 32)

	require.False(t, l.TooNarrow)
	require.Equal(t, layout.S, l.Breakpoint)
	require.Equal(t, layout.NoPreview, l.Preview)
	require.Zero(t, l.PreviewWidth)
	require.Equal(t, 2, l.RowHeight, "an S row needs two lines to carry three columns")

	names := columnNames(70)
	require.Equal(t, []string{"title", "blockers", "pipeline", "approvals", "project"}, names)
}

// SHL-15.T2. Prevents an unrecoverable render at extreme sizes.
func TestSHL15_T2_FortyColumnsIsADignifiedNotice(t *testing.T) {
	t.Parallel()

	l := layout.Compute(40, 20)

	require.True(t, l.TooNarrow)
	require.Equal(t, layout.XS, l.Breakpoint)
	require.Zero(t, l.Chrome(), "a screen that cannot draw a table draws no chrome either")
	require.Equal(t, layout.MinWidth, 56, "the notice names this width, so it is asserted here too")
}

// The three load-bearing columns. Prevents the product's whole answer — what it
// is, why it is stuck, whether it is green — being dropped to make room.
func TestSHL15_TitleBlockersAndPipelineNeverDrop(t *testing.T) {
	t.Parallel()

	for width := layout.MinWidth; width <= 220; width++ {
		require.Subset(t, columnNames(width), []string{"title", "blockers", "pipeline"},
			"a load-bearing column dropped at %d columns", width)
	}
}

// The drop order is permanent, because there is no layout setting to work
// around a bad one. Prevents it being reordered by an edit to a struct.
func TestSHL15_TheDropOrderIsTheOneThePriorityListPublishes(t *testing.T) {
	t.Parallel()

	dropped := map[string]int{}
	for _, c := range layout.Columns() {
		dropped[c.Name] = c.DropsBelow
	}

	require.Equal(t, map[string]int{
		"title": 0, "blockers": 0, "pipeline": 0,
		"approvals": 60, "project": 68, "updated": 76, "author": 90,
		"labels": 110, "diff": 124, "threads": 136, "branch": 150,
	}, dropped)
}

// Widths and bands are one specification. The band table in §8 summarises the
// drop list; the drop list is the specification, so where the two round
// differently the widths below are what a terminal actually shows.
func TestSHL15_EachWidthShowsTheColumnsTheDropListGivesIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		width int
		want  layout.Breakpoint
		last  string
	}{
		{56, layout.S, "pipeline"},
		{70, layout.S, "project"},
		{80, layout.M, "updated"},
		{119, layout.M, "labels"},
		{120, layout.L, "labels"},
		{123, layout.L, "labels"},
		{124, layout.L, "diff"},
		{159, layout.L, "branch"},
		{160, layout.XL, "branch"},
	}

	for _, tc := range cases {
		l := layout.Compute(tc.width, 32)
		require.Equal(t, tc.want, l.Breakpoint, "at %d columns", tc.width)

		names := columnNames(tc.width)
		require.Equal(t, tc.last, names[len(names)-1], "at %d columns", tc.width)
	}
}

// The chrome budget. Prevents a fifth persistent row arriving without anybody
// having to argue for it.
func TestSHL_ChromeCostsFourRows(t *testing.T) {
	t.Parallel()

	for _, width := range []int{56, 80, 120, 160, 200} {
		l := layout.Compute(width, 32)
		require.Equal(t, 4, l.Chrome(), "at %d columns", width)
		require.Equal(t, l.Height, l.Frame+l.Chrome()+l.Body,
			"rows went missing at %d columns", width)
	}
}

// Vertically, the section tabs go before any data does.
func TestSHL_TheSectionTabBarIsTheFirstThingDropped(t *testing.T) {
	t.Parallel()

	tall := layout.Compute(120, 12)
	require.Equal(t, 1, tall.SectionTabs)
	require.Equal(t, 4, tall.Chrome())

	short := layout.Compute(120, 11)
	require.Zero(t, short.SectionTabs)
	require.Equal(t, 3, short.Chrome())
	require.Positive(t, short.Body, "the tabs were dropped and there is still no room for data")
}

// The preview position is resolved, never configured. Prevents a right-hand
// pane on a 90-column terminal, which squeezes the table to uselessness.
func TestSHL06_AutoPutsThePreviewWhereTheWidthAllows(t *testing.T) {
	t.Parallel()

	require.Equal(t, layout.NoPreview, layout.Compute(70, 32).Preview)
	require.Equal(t, layout.BottomPreview, layout.Compute(119, 32).Preview)
	require.Equal(t, layout.RightPreview, layout.Compute(120, 32).Preview)

	require.Equal(t, layout.NoPreview, layout.Compute(100, 19).Preview,
		"a bottom split on a 19-row terminal leaves no table")

	require.Equal(t, 120*38/100, layout.Compute(120, 32).PreviewWidth)
	require.Equal(t, 200*34/100, layout.Compute(200, 32).PreviewWidth)
}

// Overlay width, asserted at both ends of the rule.
func TestSHL_OverlaysAreBoundedAndCentred(t *testing.T) {
	t.Parallel()

	require.Equal(t, 78, layout.Compute(200, 32).OverlayWidth)
	require.Equal(t, 52, layout.Compute(60, 32).OverlayWidth)
}

// Prevents a column being given less room than it needs to say anything, which
// is how a table starts wrapping.
func TestSHL15_NoColumnIsNarrowerThanItsMinimum(t *testing.T) {
	t.Parallel()

	for width := layout.MinWidth; width <= 220; width++ {
		l := layout.Compute(width, 32)
		widths := layout.Widths(l, 2)
		for _, c := range layout.ColumnsAt(width) {
			require.GreaterOrEqual(t, widths[c.Name], c.MinWidth,
				"%s got %d cells at %d columns", c.Name, widths[c.Name], width)
		}
	}
}

func columnNames(width int) []string {
	var names []string
	for _, c := range layout.ColumnsAt(width) {
		names = append(names, c.Name)
	}
	return names
}
