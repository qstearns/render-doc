package main

import (
	"math"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
)

type viewerModel struct {
	markdown    string
	style       glamour.TermRendererOption
	fixedWidth  int
	renderWidth int
	viewport    viewport.Model
	err         error
}

func newViewerModel(markdown string, style glamour.TermRendererOption, fixedWidth, terminalWidth int) (*viewerModel, error) {
	m := &viewerModel{
		markdown:   markdown,
		style:      style,
		fixedWidth: fixedWidth,
		viewport:   viewport.New(viewport.WithWidth(terminalWidth)),
	}
	if err := m.rerender(terminalWidth); err != nil {
		return nil, err
	}
	return m, nil
}

func viewMarkdown(markdown string, style glamour.TermRendererOption, fixedWidth int) error {
	m, err := newViewerModel(markdown, style, fixedWidth, resolveWidth(0, true))
	if err != nil {
		return err
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		return err
	}
	return m.err
}

func (m *viewerModel) Init() tea.Cmd {
	return nil
}

func (m *viewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "g", "home":
			m.viewport.GotoTop()
			return m, nil
		case "G", "shift+g", "end":
			m.viewport.GotoBottom()
			return m, nil
		}
	case tea.WindowSizeMsg:
		if err := m.resize(msg.Width, msg.Height); err != nil {
			m.err = err
			return m, tea.Quit
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *viewerModel) View() tea.View {
	v := tea.NewView(m.viewport.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *viewerModel) resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return nil
	}

	atTop := m.viewport.AtTop()
	atBottom := m.viewport.AtBottom()
	scrollPercent := m.viewport.ScrollPercent()
	xOffset := m.viewport.XOffset()

	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
	if err := m.rerender(width); err != nil {
		return err
	}

	switch {
	case atTop:
		m.viewport.GotoTop()
	case atBottom:
		m.viewport.GotoBottom()
	default:
		maxOffset := max(0, m.viewport.TotalLineCount()-m.viewport.Height())
		m.viewport.SetYOffset(int(math.Round(scrollPercent * float64(maxOffset))))
	}
	m.viewport.SetXOffset(xOffset)
	return nil
}

func (m *viewerModel) rerender(terminalWidth int) error {
	width := terminalWidth
	if m.fixedWidth > 0 {
		width = m.fixedWidth
	}
	if width == m.renderWidth {
		return nil
	}

	out, err := renderMarkdown(m.markdown, m.style, width)
	if err != nil {
		return err
	}
	m.viewport.SetContent(out)
	m.renderWidth = width
	return nil
}
