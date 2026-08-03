// Package todos is the To-Dos view.
package todos

import (
	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/tui/view"
)

// A Model is the To-Dos dashboard, S-25.
type Model struct {
	view.Base
}

func New() *Model {
	return &Model{Base: view.NewBase(
		action.ToDos, "To-Dos", action.ToDoRow,
		[]view.Section{
			{Name: "Open", Count: view.Unknown},
			{Name: "Snoozed", Count: view.Unknown},
			{Name: "Done", Count: view.Unknown},
		},
		view.Empty{
			Headline: "No To-Dos are waiting.",
			Query:    "This build draws the frame. To-Dos arrive with the data layer.",
		},
	)}
}

func (m *Model) Update(msg tea.Msg, env view.Env) (view.View, tea.Cmd) {
	base, _ := m.Base.Handle(msg, env)
	return &Model{Base: base}, nil
}
