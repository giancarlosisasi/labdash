package theme

import (
	"fmt"
	"time"

	"github.com/giancarlosisasi/labdash/internal/clock"
)

// A DateStyle is how a timestamp is written. Relative by default, because the
// question a dashboard answers about a merge request is "how stale is this",
// not "on what date did somebody touch it".
type DateStyle string

const (
	// DateRelative writes "just now", "2h ago", "3d ago".
	DateRelative DateStyle = "relative"
	// DateAbsolute writes the pattern in the configured timezone.
	DateAbsolute DateStyle = "absolute"
)

// DefaultDatePattern is a Go reference layout, so a user writes the format the
// same way the standard library documents it rather than learning strftime.
const DefaultDatePattern = "2006-01-02 15:04"

// A DateFormat renders a timestamp. It carries its own clock, so a rendered
// frame is a pure function of its inputs and a golden file containing a
// timestamp is signal rather than a flake.
type DateFormat struct {
	Style   DateStyle
	Pattern string
	Zone    *time.Location
	Clock   clock.Clock
}

func newDateFormat(opts Options) (DateFormat, error) {
	style := opts.DateFormat
	switch style {
	case "":
		style = DateRelative
	case DateRelative, DateAbsolute:
	default:
		return DateFormat{}, fmt.Errorf(
			"%q is not a date format. It is one of relative, absolute", style)
	}

	pattern := opts.DatePattern
	if pattern == "" {
		pattern = DefaultDatePattern
	}

	zone := time.Local
	if opts.Timezone != "" {
		loaded, err := time.LoadLocation(opts.Timezone)
		if err != nil {
			return DateFormat{}, fmt.Errorf(
				"%q is not a timezone this system carries. Write it as an IANA name, "+
					"for example America/Lima: %w", opts.Timezone, err)
		}
		zone = loaded
	}

	c := opts.Clock
	if c == nil {
		c = clock.System()
	}

	return DateFormat{Style: style, Pattern: pattern, Zone: zone, Clock: c}, nil
}

// Format renders t in the configured style.
func (d DateFormat) Format(t time.Time) string {
	if d.Style == DateAbsolute {
		return d.Absolute(t)
	}
	return d.Relative(t)
}

// Absolute renders t with the configured pattern, in the configured timezone.
func (d DateFormat) Absolute(t time.Time) string {
	zone := d.Zone
	if zone == nil {
		zone = time.UTC
	}
	pattern := d.Pattern
	if pattern == "" {
		pattern = DefaultDatePattern
	}
	return t.In(zone).Format(pattern)
}

// Relative renders how long ago t was, against the injected clock.
//
// The units stop at weeks: past that, "8w ago" is less use than a date, so the
// absolute form takes over. A time in the future reads "in 2h" rather than a
// negative age — a scheduled pipeline is the ordinary case, not an error.
func (d DateFormat) Relative(t time.Time) string {
	now := d.now()
	delta := now.Sub(t)

	ahead := false
	if delta < 0 {
		ahead, delta = true, -delta
	}

	unit, ok := relativeUnit(delta)
	if !ok {
		return d.Absolute(t)
	}
	if unit == "" {
		return "just now"
	}
	if ahead {
		return "in " + unit
	}
	return unit + " ago"
}

// relativeUnit returns the short form of a duration, and false when it is long
// enough that a date says more.
func relativeUnit(d time.Duration) (string, bool) {
	switch {
	case d < time.Minute:
		return "", true
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes())), true
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours())), true
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24)), true
	case d < 8*7*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7))), true
	default:
		return "", false
	}
}

func (d DateFormat) now() time.Time {
	if d.Clock == nil {
		return clock.System().Now()
	}
	return d.Clock.Now()
}
