package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// toastDismissMsg is sent after the auto-dismiss timer expires.
// The gen field ensures an old timer cannot dismiss a newer toast.
type toastDismissMsg struct{ gen int }

// Toast is a reusable TUI notification overlay that appears in the bottom-right
// corner. It auto-dismisses after 2 seconds and can be closed with the [×]
// button.
type Toast struct {
	message string
	visible bool
	gen     int

	// Populated by Render; used by HandleMouseClick for hit-testing.
	renderStartX int
	renderStartY int
	renderW      int
	renderH      int
}

// Show displays the toast with msg and returns a Cmd that schedules auto-dismiss.
func (t *Toast) Show(msg string) tea.Cmd {
	t.visible = true
	t.message = msg
	t.gen++
	gen := t.gen
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return toastDismissMsg{gen: gen}
	})
}

// Dismiss hides the toast immediately.
func (t *Toast) Dismiss() {
	t.visible = false
}

// IsVisible reports whether the toast is currently visible.
func (t *Toast) IsVisible() bool {
	return t.visible
}

// Update handles toast-related Bubble Tea messages.
func (t *Toast) Update(msg tea.Msg) tea.Cmd {
	if d, ok := msg.(toastDismissMsg); ok && d.gen == t.gen {
		t.visible = false
	}
	return nil
}

// HandleMouseClick checks whether a left-click at (screenRow, screenCol) lands
// on the [×] close button. Returns true (and dismisses) if it does.
func (t *Toast) HandleMouseClick(screenRow, screenCol int) bool {
	if !t.visible || t.renderW == 0 {
		return false
	}
	closeRow := t.renderStartY + 1 // first content row (0-indexed)
	// [×] is at the far-right of the content, give a generous hit area:
	// from (rightBorder-6) to (rightBorder-1)
	hitColStart := t.renderStartX + t.renderW - 7
	hitColEnd := t.renderStartX + t.renderW - 1
	if screenRow == closeRow && screenCol >= hitColStart && screenCol <= hitColEnd {
		t.Dismiss()
		return true
	}
	return false
}

// toastStyle returns the lipgloss style used to render the toast box.
func toastStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Background(ColorSurface).
		Foreground(ColorText).
		Padding(0, 1)
}

// Render renders the toast as a string and caches its position (computed from
// termW/termH) for subsequent mouse hit-testing. Returns "" when not visible.
// The margin constants must match those used in placeOverlayBottomRight.
func (t *Toast) Render(termW, termH int) string {
	if !t.visible {
		return ""
	}

	closeBtn := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Background(ColorSurface).
		Render("[×]")

	icon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Background(ColorSurface).
		Render("✓")

	contentLine := icon + " " + t.message + "  " + closeBtn

	box := toastStyle().Render(contentLine)

	t.renderW = lipgloss.Width(box)
	t.renderH = strings.Count(box, "\n") + 1

	const margin = 2
	t.renderStartX = termW - t.renderW - margin
	t.renderStartY = termH - t.renderH - margin

	return box
}
