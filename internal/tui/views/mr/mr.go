// Package mr is the merge requests view.
package mr

import (
	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/tui/view"
)

// A Model is the merge requests dashboard, S-07.
type Model struct {
	view.Base
}

func New() *Model {
	return &Model{Base: view.NewBase(
		action.MergeRequests, "Merge requests", action.MergeRequestRow,
		[]view.Section{
			{Name: "Needs my review", Count: view.Unknown},
			{Name: "Mine", Count: view.Unknown},
			{Name: "Everything", Count: view.Unknown},
		},
		view.Empty{
			Headline: "Nothing needs your review.",
			Query:    "This build draws the frame. Merge requests arrive with the data layer.",
		},
	)}
}

func (m *Model) Update(msg tea.Msg, env view.Env) (view.View, tea.Cmd) {
	base, _ := m.Base.Handle(msg, env)
	return &Model{Base: base}, nil
}
