// Package issues is the issues view.
package issues

import (
	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/tui/view"
)

// A Model is the issues dashboard.
type Model struct {
	view.Base
}

func New() *Model {
	return &Model{Base: view.NewBase(
		action.Issues, "Issues", action.IssueRow,
		[]view.Section{
			{Name: "Assigned to me", Count: view.Unknown},
			{Name: "Everything", Count: view.Unknown},
		},
		view.Empty{
			Headline: "No issues are assigned to you.",
			Query:    "This build draws the frame. Issues arrive with the data layer.",
		},
	)}
}

func (m *Model) Update(msg tea.Msg, env view.Env) (view.View, tea.Cmd) {
	base, _ := m.Base.Handle(msg, env)
	return &Model{Base: base}, nil
}
