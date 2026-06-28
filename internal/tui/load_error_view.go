package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// loadErrorReservedLines is the overhead for title, help, and padding.
const loadErrorReservedLines = 13

// loadErrorMaxVisible returns the number of message lines that fit.
func (m *Model) loadErrorMaxVisible() int {
	v := m.height - loadErrorReservedLines
	if v < MinViewHeight {
		return MinViewHeight
	}
	return v
}

// loadErrorWrappedLines returns the message hard-wrapped to the content width so
// long paths wrap instead of running off the screen.
func (m *Model) loadErrorWrappedLines() []string {
	width := m.width - BaseStylePadding
	if !m.sizeKnown || width < 1 {
		return m.loadErrLines
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(strings.Join(m.loadErrLines, "\n"))
	lines := strings.Split(wrapped, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// openLoadError opens the detail screen for the load-failed plugin under the
// cursor, if any.
func (m *Model) openLoadError() {
	if m.listScroll.cursor < 0 || m.listScroll.cursor >= len(m.plugins) {
		return
	}
	p := m.plugins[m.listScroll.cursor]
	if p.Status != StatusLoadFailed {
		return
	}
	m.loadErrName = p.Name
	// TrimRight drops the trailing blank line from a message ending in "\n".
	m.loadErrLines = strings.Split(strings.TrimRight(p.LoadErr, "\n"), "\n")
	m.loadErrScroll.reset()
	m.screen = ScreenLoadError
}

// handleKeyMsgLoadError handles key events on the load-error detail screen.
func (m Model) handleKeyMsgLoadError(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, SharedKeys.Quit), msg.String() == escKeyName:
		m.screen = ScreenList
		m.loadErrName = ""
		m.loadErrLines = nil
		m.loadErrScroll.reset()
	case key.Matches(msg, ListKeys.Up):
		m.loadErrScroll.moveUp()
	case key.Matches(msg, ListKeys.Down):
		m.loadErrScroll.moveDown(len(m.loadErrorWrappedLines()), m.loadErrorMaxVisible())
	}
	return m, nil
}

// viewLoadError renders the scrollable load-error detail screen.
func (m *Model) viewLoadError() string {
	var b strings.Builder

	title := fmt.Sprintf("  ✗ %s — loading error  ", m.loadErrName)
	b.WriteString(m.centerText(m.theme.TitleStyle.Render(title)))
	b.WriteString("\n\n")

	lines := m.loadErrorWrappedLines()
	maxVisible := m.loadErrorMaxVisible()
	end := min(m.loadErrScroll.scrollOffset+maxVisible, len(lines))
	top, bottom, dataStart, dataEnd := m.theme.renderScrollIndicators(m.loadErrScroll.scrollOffset, end, len(lines))
	b.WriteString(top)
	for i := dataStart; i < dataEnd; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	b.WriteString(bottom)

	help := m.centerText(m.theme.renderHelp(m.width, SharedKeys.Back, SharedKeys.Quit))
	return padToBottom(b.String(), help, m.height)
}
