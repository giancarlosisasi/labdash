// Package pipelines is the pipelines view.
package pipelines

import (
	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/tui/view"
)

// A Model is the pipelines dashboard, S-08. It is deliberately the same
// skeleton as merge requests, so the keys and the spatial model transfer with
// nothing to relearn.
type Model struct {
	view.Base
}

func New() *Model {
	return &Model{Base: view.NewBase(
		action.Pipelines, "Pipelines", action.PipelineRow,
		[]view.Section{
			{Name: "Failed today", Count: view.Unknown},
			{Name: "Running", Count: view.Unknown},
			{Name: "Everything", Count: view.Unknown},
		},
		view.Empty{
			Headline: "No pipelines to show.",
			Query:    "This build draws the frame. Pipelines arrive with the data layer.",
		},
	)}
}

func (m *Model) Update(msg tea.Msg, env view.Env) (view.View, tea.Cmd) {
	base, _ := m.Base.Handle(msg, env)
	return &Model{Base: base}, nil
}
