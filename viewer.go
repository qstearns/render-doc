package main

import (
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
)

// multiClickInterval is how long a second or third click still counts as part
// of the same gesture.
const multiClickInterval = 400 * time.Millisecond

type viewerModel struct {
	markdown    string
	style       glamour.TermRendererOption
	fixedWidth  int
	renderWidth int
	lines       []string // the rendered document, one line per element
	viewport    viewport.Model
	selection   selection
	lastClick   cell
	lastClickAt time.Time
	clicks      int
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
		case "y":
			return m, m.copySelection()
		case "esc":
			m.selection = selection{}
			return m, nil
		}
	case tea.MouseClickMsg:
		m.mouseDown(msg.Mouse())
		return m, nil
	case tea.MouseMotionMsg:
		m.mouseDrag(msg.Mouse())
		return m, nil
	case tea.MouseReleaseMsg:
		return m, m.mouseUp(msg.Mouse())
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
	rows := strings.Split(m.viewport.View(), "\n")
	m.selection.highlight(rows, m.viewport.YOffset(), m.viewport.XOffset(), m.viewport.Width())

	v := tea.NewView(strings.Join(rows, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// mouseDown starts a selection. Repeated clicks on the same cell widen the
// gesture from a drag to a word to the whole line, as they do in a terminal;
// a fourth click starts over.
func (m *viewerModel) mouseDown(e tea.Mouse) {
	if e.Button != tea.MouseLeft {
		return
	}

	at := m.cellAt(e.X, e.Y)
	now := time.Now()
	if at == m.lastClick && now.Sub(m.lastClickAt) < multiClickInterval {
		m.clicks = m.clicks%3 + 1
	} else {
		m.clicks = 1
	}
	m.lastClick, m.lastClickAt = at, now

	switch m.clicks {
	case 2:
		if sel := m.selectWord(at); sel.active {
			m.selection = sel
			return
		}
	case 3:
		if sel := m.selectLine(at); sel.active {
			m.selection = sel
			return
		}
	}
	// A click on its own clears the selection, as does one that landed
	// somewhere with no word or line to take; it only becomes a selection once
	// the pointer moves with the button down.
	m.selection = selection{anchor: at, cursor: at, drag: true}
}

func (m *viewerModel) mouseDrag(e tea.Mouse) {
	if !m.selection.drag {
		return
	}
	// Dragging against the top or bottom edge scrolls, so a selection can run
	// past what fits on screen.
	switch {
	case e.Y <= 0:
		m.viewport.ScrollUp(1)
	case e.Y >= m.viewport.Height()-1:
		m.viewport.ScrollDown(1)
	}
	m.selection.cursor = m.cellAt(e.X, e.Y)
	m.selection.active = true
}

func (m *viewerModel) mouseUp(e tea.Mouse) tea.Cmd {
	if e.Button != tea.MouseLeft {
		return nil
	}
	m.selection.drag = false
	return m.copySelection()
}

// copySelection puts the selected text on the system clipboard with OSC 52,
// which is the only route that also works over ssh. Terminals without support
// for the escape (Terminal.app, notably) drop it silently.
func (m *viewerModel) copySelection() tea.Cmd {
	text := m.selection.text(m.lines)
	if text == "" {
		return nil
	}
	return tea.SetClipboard(text)
}

// cellAt maps a position on screen to one in the document. The viewport fills
// the window and doesn't soft wrap, so a screen row is a document line and a
// screen column a document column, each shifted by the scroll offset.
func (m *viewerModel) cellAt(x, y int) cell {
	if len(m.lines) == 0 || m.viewport.Width() <= 0 || m.viewport.Height() <= 0 {
		return cell{}
	}
	return cell{
		line: min(m.viewport.YOffset()+min(max(y, 0), m.viewport.Height()-1), len(m.lines)-1),
		col:  m.viewport.XOffset() + min(max(x, 0), m.viewport.Width()-1),
	}
}

func (m *viewerModel) selectWord(at cell) selection {
	if at.line >= len(m.lines) {
		return selection{}
	}
	from, to, ok := wordAt(ansi.Strip(m.lines[at.line]), at.col)
	if !ok {
		return selection{}
	}
	return selection{
		anchor: cell{line: at.line, col: from},
		cursor: cell{line: at.line, col: to - 1},
		active: true,
	}
}

func (m *viewerModel) selectLine(at cell) selection {
	if at.line >= len(m.lines) {
		return selection{}
	}
	width := ansi.StringWidth(strings.TrimRight(ansi.Strip(m.lines[at.line]), " "))
	if width == 0 {
		return selection{}
	}
	return selection{
		anchor: cell{line: at.line, col: 0},
		cursor: cell{line: at.line, col: width - 1},
		active: true,
	}
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
	m.lines = strings.Split(out, "\n")
	// Reflowed text moves out from under the selection's coordinates.
	m.selection = selection{}
	m.renderWidth = width
	return nil
}
