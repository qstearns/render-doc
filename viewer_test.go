package main

import (
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
