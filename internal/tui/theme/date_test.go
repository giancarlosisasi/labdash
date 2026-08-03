package theme_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/clock"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// now is the instant every date test is measured against.
var now = time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)

// THM-10.T1, the relative half. Prevents: a "5 hours ago" that is wrong for
// everyone outside UTC, and a relative form that reads a real clock and so can
// never be put in a golden file.
func TestTHM10_T1_RelativeForm(t *testing.T) {
	t.Parallel()

	d := dates(t, theme.Options{DateFormat: theme.DateRelative, Timezone: "UTC"})

	for _, tc := range []struct {
		at   time.Time
		want string
	}{
		{now, "just now"},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-time.Minute), "1m ago"},
		{now.Add(-45 * time.Minute), "45m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-23 * time.Hour), "23h ago"},
		{now.Add(-3 * 24 * time.Hour), "3d ago"},
		{now.Add(-6 * 24 * time.Hour), "6d ago"},
		{now.Add(-14 * 24 * time.Hour), "2w ago"},

		// A scheduled pipeline is ahead of now. It reads as a time, not as a
		// negative age.
		{now.Add(2 * time.Hour), "in 2h"},
		{now.Add(15 * time.Minute), "in 15m"},
	} {
		require.Equal(t, tc.want, d.Relative(tc.at), "for %s", tc.at)
	}
}

// Prevents: "9w ago" — past a couple of months a date says more than a count of
// weeks, so the relative form hands over to the absolute one.
func TestRelativeHandsOverToADateWhenItStopsHelping(t *testing.T) {
	t.Parallel()

	d := dates(t, theme.Options{DateFormat: theme.DateRelative, Timezone: "UTC"})

	require.Equal(t, "2026-06-07 14:30", d.Relative(now.Add(-56*24*time.Hour)))
	require.Equal(t, "2025-08-02 14:30", d.Relative(now.AddDate(-1, 0, 0)))
}

// THM-10.T1, the absolute half. Prevents: a pattern or a timezone that is
// accepted and then ignored.
func TestTHM10_T1_AbsoluteFormHonoursPatternAndZone(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)

	for _, tc := range []struct {
		zone, pattern, want string
	}{
		{"UTC", "2006-01-02 15:04", "2026-08-02 14:30"},
		{"America/Lima", "2006-01-02 15:04", "2026-08-02 09:30"},
		{"Asia/Tokyo", "2006-01-02 15:04", "2026-08-02 23:30"},
		{"America/Lima", "Mon 2 Jan 15:04", "Sun 2 Aug 09:30"},
		{"UTC", time.RFC3339, "2026-08-02T14:30:00Z"},
	} {
		d := dates(t, theme.Options{
			DateFormat: theme.DateAbsolute, Timezone: tc.zone, DatePattern: tc.pattern,
		})
		require.Equal(t, tc.want, d.Format(at), "in %s with %q", tc.zone, tc.pattern)
	}
}

// Prevents: an off-by-one-hour bug twice a year. Peru does not observe daylight
// saving; Madrid does, and a timestamp either side of the change must land on
// the right wall-clock hour.
func TestAbsoluteFormSurvivesADaylightSavingBoundary(t *testing.T) {
	t.Parallel()

	d := dates(t, theme.Options{
		DateFormat: theme.DateAbsolute, Timezone: "Europe/Madrid",
		DatePattern: "2006-01-02 15:04 MST",
	})

	// Central European Summer Time ends at 03:00 local on 25 October 2026.
	require.Equal(t, "2026-10-25 02:59 CEST",
		d.Format(time.Date(2026, 10, 25, 0, 59, 0, 0, time.UTC)))
	require.Equal(t, "2026-10-25 02:00 CET",
		d.Format(time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)))
}

// THM-10.T1, the determinism half. Prevents: a golden file containing a
// timestamp becoming a flake instead of a test.
func TestTHM10_T1_OutputIsByteIdenticalAcrossRenders(t *testing.T) {
	t.Parallel()

	at := now.Add(-2 * time.Hour)

	for _, style := range []theme.DateStyle{theme.DateRelative, theme.DateAbsolute} {
		d := dates(t, theme.Options{DateFormat: style, Timezone: "America/Lima"})

		first := d.Format(at)
		time.Sleep(2 * time.Millisecond)
		for range 50 {
			require.Equal(t, first, d.Format(at),
				"the %s form changed between two renders of the same instant", style)
		}
	}
}

// Prevents: an unusable timezone or format name failing with a Go error rather
// than with what to write instead.
func TestUnusableDateSettingsAreExplained(t *testing.T) {
	t.Parallel()

	_, err := theme.New(theme.Options{Env: termcap.TrueColor, Timezone: "Mars/Olympus"})
	require.ErrorContains(t, err, "IANA")
	require.ErrorContains(t, err, "America/Lima")

	_, err = theme.New(theme.Options{Env: termcap.TrueColor, DateFormat: "epoch"})
	require.ErrorContains(t, err, "relative, absolute")
}

// Prevents: the default drifting. Relative is the default because the question
// a dashboard answers about a merge request is how stale it is.
func TestTheDefaultIsRelative(t *testing.T) {
	t.Parallel()

	d := dates(t, theme.Options{})
	require.Equal(t, theme.DateRelative, d.Style)
	require.Equal(t, theme.DefaultDatePattern, d.Pattern)
}

func dates(t *testing.T, opts theme.Options) theme.DateFormat {
	t.Helper()
	opts.Env = termcap.TrueColor
	opts.Clock = clock.Fixed(now)
	return build(t, opts).Dates
}
