package main

import (
	"fmt"
	"math"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
)

func testViewer(t *testing.T, markdown string, fixedWidth, terminalWidth, terminalHeight int) *viewerModel {
	t.Helper()
	m, err := newViewerModel(
		markdown,
		glamour.WithStandardStyle(styles.NoTTYStyle),
		fixedWidth,
		terminalWidth,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.resize(terminalWidth, terminalHeight); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestViewerReflowsOnResize(t *testing.T) {
	markdown := strings.Repeat("word ", 80)
	m := testViewer(t, markdown, 0, 80, 10)
	wideContent := m.viewport.GetContent()
	wideLines := m.viewport.TotalLineCount()

	_, cmd := m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	if cmd != nil {
		t.Fatal("resize unexpectedly returned a command")
	}
	if m.renderWidth != 30 {
		t.Fatalf("render width = %d, want 30", m.renderWidth)
	}
	if m.viewport.GetContent() == wideContent {
		t.Fatal("content was not rerendered")
	}
	if got := m.viewport.TotalLineCount(); got <= wideLines {
		t.Fatalf("narrow line count = %d, want more than %d", got, wideLines)
	}
}

func TestViewerKeepsExplicitWidthOnResize(t *testing.T) {
	markdown := strings.Repeat("word ", 80)
	m := testViewer(t, markdown, 60, 80, 10)
	content := m.viewport.GetContent()

	_, cmd := m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	if cmd != nil {
		t.Fatal("resize unexpectedly returned a command")
	}
	if m.renderWidth != 60 {
		t.Fatalf("render width = %d, want 60", m.renderWidth)
	}
	if m.viewport.Width() != 30 {
		t.Fatalf("viewport width = %d, want 30", m.viewport.Width())
	}
	if got := m.viewport.GetContent(); got != content {
		t.Fatal("fixed-width content changed after terminal resize")
	}
}

func TestViewerPreservesScrollPositionOnReflow(t *testing.T) {
	markdown := strings.Repeat("A paragraph with enough words to wrap over several lines at narrow widths.\n\n", 40)
	m := testViewer(t, markdown, 0, 70, 8)
	maxOffset := m.viewport.TotalLineCount() - m.viewport.Height()
	if maxOffset < 4 {
		t.Fatalf("test document is only %d scrollable lines", maxOffset)
	}
	m.viewport.SetYOffset(maxOffset / 2)
	want := m.viewport.ScrollPercent()

	_, cmd := m.Update(tea.WindowSizeMsg{Width: 35, Height: 8})
	if cmd != nil {
		t.Fatal("resize unexpectedly returned a command")
	}
	if got := m.viewport.ScrollPercent(); math.Abs(got-want) > 0.02 {
		t.Fatalf("scroll position = %.3f, want approximately %.3f", got, want)
	}
}

func TestViewerPagerKeys(t *testing.T) {
	m := testViewer(t, strings.Repeat("line\n\n", 30), 0, 40, 5)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "G", Code: 'g', ShiftedCode: 'G'})
	if cmd != nil {
		t.Fatal("G unexpectedly returned a command")
	}
	if !m.viewport.AtBottom() {
		t.Fatal("G did not jump to the bottom")
	}

	_, cmd = m.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
	if cmd == nil {
		t.Fatal("q did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q command returned %T, want tea.QuitMsg", cmd())
	}
}

// clipboardText reads back what a command would put on the clipboard.
// tea.SetClipboard's message type is unexported, but it's a string underneath.
func clipboardText(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		return ""
	}
	return fmt.Sprint(cmd())
}

func click(m *viewerModel, x, y int) {
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func drag(m *viewerModel, x, y int) {
	m.Update(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func release(m *viewerModel, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	return cmd
}

// selectionViewer renders a document whose second line is the paragraph
// "  hello world and more  ", padded out to the viewer's width.
func selectionViewer(t *testing.T) *viewerModel {
	t.Helper()
	return testViewer(t, "hello world and more", 0, 40, 6)
}

func TestViewerDragSelectsAndCopies(t *testing.T) {
	m := selectionViewer(t)

	click(m, 2, 1)
	drag(m, 6, 1)
	cmd := release(m, 6, 1)

	if got := clipboardText(t, cmd); got != "hello" {
		t.Errorf("copied %q, want %q", got, "hello")
	}
	if want := "\x1b[7mhello\x1b[m"; !strings.Contains(m.View().Content, want) {
		t.Errorf("view does not highlight the selection: %q", m.View().Content)
	}
}

func TestViewerClickWithoutDragClearsSelection(t *testing.T) {
	m := selectionViewer(t)

	click(m, 2, 1)
	drag(m, 6, 1)
	release(m, 6, 1)

	click(m, 20, 1)
	if cmd := release(m, 20, 1); cmd != nil {
		t.Errorf("a click on its own copied %q", clipboardText(t, cmd))
	}
	if strings.Contains(m.View().Content, "\x1b[7m") {
		t.Error("a click on its own left the selection highlighted")
	}
}

func TestViewerMultiClickSelectsWordThenLine(t *testing.T) {
	m := selectionViewer(t)

	click(m, 9, 1)
	click(m, 9, 1)
	if got := clipboardText(t, release(m, 9, 1)); got != "world" {
		t.Errorf("double click copied %q, want %q", got, "world")
	}

	click(m, 9, 1)
	if got := clipboardText(t, release(m, 9, 1)); got != "  hello world and more" {
		t.Errorf("triple click copied %q, want the whole line", got)
	}
}

func TestViewerCopyAndClearKeys(t *testing.T) {
	m := selectionViewer(t)

	click(m, 2, 1)
	drag(m, 6, 1)
	release(m, 6, 1)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "y", Code: 'y'})
	if got := clipboardText(t, cmd); got != "hello" {
		t.Errorf("y copied %q, want %q", got, "hello")
	}

	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd != nil {
		t.Fatal("esc unexpectedly returned a command")
	}
	if strings.Contains(m.View().Content, "\x1b[7m") {
		t.Error("esc left the selection highlighted")
	}
}

func TestViewerDragPastTheBottomScrolls(t *testing.T) {
	m := testViewer(t, strings.Repeat("paragraph\n\n", 20), 0, 40, 5)

	click(m, 2, 1)
	for range 3 {
		drag(m, 2, m.viewport.Height()-1)
	}

	if got := m.viewport.YOffset(); got != 3 {
		t.Fatalf("y offset = %d, want 3", got)
	}
	if got := clipboardText(t, release(m, 2, m.viewport.Height()-1)); strings.Count(got, "\n") < 3 {
		t.Errorf("selection stopped at the screen edge: %q", got)
	}
}

func TestViewerReflowClearsSelection(t *testing.T) {
	m := selectionViewer(t)

	click(m, 2, 1)
	drag(m, 6, 1)
	release(m, 6, 1)

	m.Update(tea.WindowSizeMsg{Width: 20, Height: 6})
	if m.selection.active {
		t.Error("selection survived a reflow, where its coordinates no longer mean the same thing")
	}
}

func TestViewerDoubleClickOnBlankSpaceStillDrags(t *testing.T) {
	m := selectionViewer(t)

	click(m, 0, 1)
	click(m, 0, 1) // the left margin has no word to take
	drag(m, 6, 1)

	if got := clipboardText(t, release(m, 6, 1)); got != "  hello" {
		t.Errorf("copied %q, want %q", got, "  hello")
	}
}
