// Package action answers one question about every key labdash binds: can this
// run, here, now — and if not, why not.
//
// There is exactly one predicate per action, and the footer, the `?` overlay,
// the command palette and the confirm layer all read it. A surface that
// computes its own answer is how "why can't I do this" grows three different
// replies, and with the keymap fixed and nothing rebindable, a keypress that
// answers nothing is a defect rather than a rough edge.
//
// The reason is written for the person at the keyboard. It is a clause, not a
// sentence: the caller renders "Cannot merge — " in front of it.
//
// It exists before the actions it guards on purpose. Adding it now costs a few
// lines; retrofitting it later means touching every action site.
package action

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// A Screen is a place a key can be bound: one of the dashboard views, or one of
// the screens reached from them.
type Screen string

const (
	// Everywhere is the universal context, merged into every screen below.
	Everywhere Screen = "everywhere"

	MergeRequests Screen = "merge requests"
	Pipelines     Screen = "pipelines"
	ToDos         Screen = "to-dos"
	Issues        Screen = "issues"
	Browse        Screen = "browse"
	Trace         Screen = "job trace"
	// SignIn is the first-run flow. It is a screen like any other, so its keys
	// live in the same table and answer under the same predicate.
	SignIn Screen = "sign in"
)

// Screens is every context a key resolves in, universal excluded, in the order
// /keys/ prints them.
func Screens() []Screen {
	return []Screen{SignIn, Browse, MergeRequests, Pipelines, Trace, ToDos, Issues}
}

// Views is the subset the view switcher cycles through. Browse and the trace
// viewer are reached from a view, never by Tab.
func Views() []Screen { return []Screen{MergeRequests, Pipelines, ToDos, Issues} }

// Title is the heading a screen appears under.
func (s Screen) Title() string {
	if s == ToDos {
		return "To-Dos"
	}
	r := []rune(string(s))
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

// A RowKind is what the selected row is. Naming the kind an action accepts is
// enough to answer "this row cannot take it" without the action knowing
// anything else about the row.
type RowKind string

const (
	NoRow RowKind = ""

	MergeRequestRow RowKind = "merge request"
	PipelineRow     RowKind = "pipeline"
	JobRow          RowKind = "job"
	ToDoRow         RowKind = "To-Do"
	IssueRow        RowKind = "issue"
	TreeNodeRow     RowKind = "group"
	LogLineRow      RowKind = "log line"
)

// A Row is the selected row, reduced to what availability needs from it. It
// grows a field when an action needs one, and not before.
type Row struct {
	Kind RowKind
}

// A Scope is what the credential may do, under GitLab's own name for it: a user
// who reads "read_api" here must find the same word in GitLab's settings.
type Scope string

const (
	ScopeNone  Scope = ""
	ScopeRead  Scope = "read_api"
	ScopeWrite Scope = "api"
)

// A Context is everything a predicate may look at.
type Context struct {
	Screen     Screen
	Row        Row
	Scope      Scope
	SearchLive bool
	Offline    bool
}

// An Availability is the answer. Reason is a lowercase clause the caller
// renders after "Cannot <verb> — ", and is empty exactly when OK.
type Availability struct {
	OK     bool
	Reason string
}

var Yes = Availability{OK: true}

func No(reason string) Availability { return Availability{Reason: reason} }

// A Predicate is the one answer for one action.
type Predicate func(Context) Availability

// A Requirement is what an action needs in order to run. An action states its
// needs and never writes its own refusal, which is how forty actions avoid
// forty phrasings of the same sentence.
type Requirement struct {
	// Built reports that the action exists in this build. A declared but
	// unbuilt action is still bound, still on /keys/, and still answers when
	// pressed — the whole reason the keymap is declared whole.
	Built bool
	// Screens limits the action further than the binding already does. Empty
	// means every screen it is bound on.
	Screens []Screen
	// Row is the kind of row the action acts on; NoRow needs no selection.
	Row RowKind
	// Write is the whole of read-only mode. There is no separate branch for it
	// anywhere else.
	Write      bool
	Network    bool
	SearchLive bool
}

// Require builds the one predicate for a requirement.
//
// The checks run in the order a person would ask the questions, so the reason
// they see is the nearest one: does this exist, is it offered here, is there
// something to do it to, may this token do it, is the instance reachable.
func Require(r Requirement) Predicate {
	return func(ctx Context) Availability {
		if !r.Built {
			return No("it is not in this release yet")
		}

		if len(r.Screens) > 0 && !slices.Contains(r.Screens, ctx.Screen) {
			return No(fmt.Sprintf("it belongs to %s, not to %s",
				join(r.Screens), string(ctx.Screen)))
		}

		if r.SearchLive && !ctx.SearchLive {
			return No("no search is running — press / to start one")
		}

		if r.Row != NoRow {
			switch {
			case ctx.Row.Kind == NoRow:
				return No("nothing is selected")
			case ctx.Row.Kind != r.Row:
				return No(fmt.Sprintf("the selected row is a %s, not a %s",
					string(ctx.Row.Kind), string(r.Row)))
			}
		}

		if r.Write {
			switch ctx.Scope {
			case ScopeNone:
				return No("you are not signed in to GitLab yet")
			case ScopeRead:
				return No("this token is read-only, so labdash cannot change anything")
			}
		}

		if r.Network && ctx.Offline {
			return No("the instance is unreachable, so nothing can be sent")
		}

		return Yes
	}
}

// Sentence is the refusal every surface shows, so the footer, the overlay and
// the palette cannot phrase one refusal three ways.
func Sentence(verb string, a Availability) string {
	if a.OK {
		return ""
	}
	return fmt.Sprintf("Cannot %s — %s.", verb, a.Reason)
}

func join(screens []Screen) string {
	names := make([]string, len(screens))
	for i, s := range screens {
		names[i] = string(s)
	}
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}
