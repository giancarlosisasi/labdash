package action_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
)

// SHL-24.T1. Prevents three divergent answers to "why can't I do this", and a
// fixed keymap where a key can be pressed and answer nothing.
//
// Every action is put against a row it cannot act on, a screen that does not
// offer it, and a read_api token. Each has to refuse with a reason a person can
// act on.
func TestSHL24_T1_EveryActionRefusesWithAReason(t *testing.T) {
	t.Parallel()

	for _, e := range keymap.Table() {
		t.Run(string(e.ID), func(t *testing.T) {
			t.Parallel()

			cases := refusalCases(e)
			if len(cases) == 0 {
				require.Equal(t, action.Everywhere, e.Screen,
					"%s owes a refusal for the screens that do not offer it", e.ID)
				require.Equal(t, action.Requirement{}, e.Requires,
					"%s needs something, so it owes a refusal for the case where it is missing", e.ID)
				return
			}

			// Every case below removes exactly one thing the action needs,
			// against a build that has it, so the reason names that one thing
			// rather than the release it is missing from.
			built := e
			built.Implemented = true

			for _, tc := range cases {
				got := built.Available(tc.ctx)
				if tc.name == "an unbuilt action" {
					got = e.Available(tc.ctx)
				}
				require.False(t, got.OK, "%s answered yes to %s", e.ID, tc.name)
				requireUserFacing(t, e, got.Reason, tc.name)
			}
		})
	}
}

type refusal struct {
	name string
	ctx  action.Context
}

// refusalCases builds the three refusals every action owes an answer to, using
// the screen the binding actually resolves on so that "unavailable here" is not
// standing in for "unavailable at all".
func refusalCases(e keymap.Entry) []refusal {
	home := e.Screen
	if home == action.Everywhere {
		home = action.MergeRequests
	}

	live := action.Context{
		Screen:     home,
		Row:        action.Row{Kind: rowForScreen(home)},
		Scope:      action.ScopeWrite,
		SearchLive: true,
	}

	var cases []refusal
	if !e.Implemented {
		cases = append(cases, refusal{"an unbuilt action", live})
	}

	if e.Requires.Row != action.NoRow {
		empty := live
		empty.Row = action.Row{}
		wrong := live
		wrong.Row = action.Row{Kind: action.LogLineRow}
		cases = append(cases,
			refusal{"an empty table", empty},
			refusal{"a row of the wrong kind", wrong})
	}
	if e.Requires.Write {
		readOnly := live
		readOnly.Scope = action.ScopeRead
		signedOut := live
		signedOut.Scope = action.ScopeNone
		cases = append(cases,
			refusal{"a read_api token", readOnly},
			refusal{"no credential at all", signedOut})
	}
	if e.Requires.Network {
		offline := live
		offline.Offline = true
		cases = append(cases, refusal{"an unreachable instance", offline})
	}
	if e.Requires.SearchLive {
		noSearch := live
		noSearch.SearchLive = false
		cases = append(cases, refusal{"no search running", noSearch})
	}
	if e.Screen != action.Everywhere {
		elsewhere := live
		elsewhere.Screen = otherScreen(e.Screen)
		cases = append(cases, refusal{"a screen that does not offer it", elsewhere})
	}
	return cases
}

// requireUserFacing is the "no internal identifier, error type, or stack" half
// of SHL-24. A reason that leaks a Go type is a reason nobody can act on.
func requireUserFacing(t *testing.T, e keymap.Entry, reason, when string) {
	t.Helper()

	require.NotEmpty(t, reason, "%s refused %s with no reason", e.ID, when)
	require.NotContains(t, reason, "\n", "%s wrote a multi-line reason", e.ID)
	require.True(t, unicode.IsLower([]rune(reason)[0]),
		"%s starts its reason with a capital; it is a clause, not a sentence: %q", e.ID, reason)

	for _, leak := range []string{"0x", "%!", "nil", "panic", "error", "Error", "action.", "*"} {
		require.NotContains(t, reason, leak,
			"%s leaks %q into a user-facing reason: %q", e.ID, leak, reason)
	}
	if strings.Contains(string(e.ID), "-") {
		// A one-word id such as "merge" is also an English word, and refusing
		// to say "merge request" would be the wrong lesson to draw.
		require.NotContains(t, reason, string(e.ID),
			"%s puts its own identifier in the reason: %q", e.ID, reason)
	}

	sentence := action.Sentence(string(e.Verb), action.Availability{Reason: reason})
	require.True(t, strings.HasPrefix(sentence, "Cannot "), "the refusal is not a sentence: %q", sentence)
	require.True(t, strings.HasSuffix(sentence, "."), "the refusal has no full stop: %q", sentence)
}

// Prevents: a predicate that refuses everything, which would pass every
// assertion above and make the whole product unusable.
func TestSHL24_AnImplementedActionSaysYesWhenEverythingIsInPlace(t *testing.T) {
	t.Parallel()

	var checked int
	for _, e := range keymap.Table() {
		if !e.Implemented {
			continue
		}
		checked++

		screen := e.Screen
		if screen == action.Everywhere {
			screen = action.MergeRequests
		}
		got := e.Available(action.Context{
			Screen:     screen,
			Row:        action.Row{Kind: rowForScreen(screen)},
			Scope:      action.ScopeWrite,
			SearchLive: true,
		})
		require.True(t, got.OK, "%s refused a context that satisfies it: %s", e.ID, got.Reason)
	}
	require.NotZero(t, checked, "nothing is implemented, so the assertion proves nothing")
}

// Read-only mode is a predicate rather than a special case. AUT-07 has no
// branch of its own anywhere; it is this one field.
func TestSHL24_ReadOnlyIsTheOrdinaryPredicatePath(t *testing.T) {
	t.Parallel()

	readOnly := action.Context{
		Screen: action.MergeRequests,
		Row:    action.Row{Kind: action.MergeRequestRow},
		Scope:  action.ScopeRead,
	}

	var mutating int
	for _, e := range keymap.For(action.MergeRequests) {
		if !e.Requires.Write {
			continue
		}
		mutating++
		built := e
		built.Implemented = true
		got := built.Available(readOnly)
		require.False(t, got.OK, "%s is available with a read-only token", e.ID)
		require.Contains(t, got.Reason, "read-only")
	}
	require.NotZero(t, mutating, "merge requests declares no mutating action")
}

func rowForScreen(s action.Screen) action.RowKind {
	switch s {
	case action.Pipelines:
		return action.PipelineRow
	case action.ToDos:
		return action.ToDoRow
	case action.Issues:
		return action.IssueRow
	case action.Browse:
		return action.TreeNodeRow
	case action.Trace:
		return action.JobRow
	default:
		return action.MergeRequestRow
	}
}

func otherScreen(s action.Screen) action.Screen {
	if s == action.Pipelines {
		return action.MergeRequests
	}
	return action.Pipelines
}
