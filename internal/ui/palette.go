package ui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Theme Colors
	ColorBg      = lipgloss.Color("#0C0C0C")
	ColorSurface = lipgloss.Color("#242424")
	ColorText    = lipgloss.Color("#E0E0E0")
	ColorMuted   = lipgloss.Color("#707070")
	ColorAccent  = lipgloss.Color("#FFB38A") // Peach accent
	ColorHeader  = lipgloss.Color("#8B89E1") // Periwinkle
	ColorBorder  = lipgloss.Color("#3C3C3C")

	// Styles
	normalTitleStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				PaddingLeft(3)

	selectedTitleStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(lipgloss.Color("#000000")).
				PaddingLeft(3).
				Bold(true)

	normalDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingLeft(3)

	selectedDescStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(lipgloss.Color("#202020")).
				PaddingLeft(3)

	shortcutStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingRight(3).
			Align(lipgloss.Right)

	selectedShortcutStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(lipgloss.Color("#202020")).
				PaddingRight(3).
				Align(lipgloss.Right)

	categoryStyle = lipgloss.NewStyle().
			Foreground(ColorHeader).
			PaddingLeft(1).
			PaddingTop(1).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
)

type item struct {
	title, desc string
	shortcut    string
	category    string
	hasActions  bool
	meta        map[string]interface{}
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title + " " + i.category }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	
	var style, shortcutS lipgloss.Style

	if isSelected {
		style = selectedTitleStyle
		shortcutS = selectedShortcutStyle
	} else {
		style = normalTitleStyle
		shortcutS = shortcutStyle
	}

	// Calculate width for shortcut
	availWidth := m.Width()
	title := i.title
	
	line := style.Width(availWidth).Render(title)
	
	if i.shortcut != "" {
		s := shortcutS.Render(i.shortcut)
		sw := lipgloss.Width(s)
		
		if availWidth > sw+5 {
			// We use Horizontal Join to stick the shortcut to the right
			line = lipgloss.JoinHorizontal(lipgloss.Left,
				style.Width(availWidth-sw).Render(title),
				s,
			)
		}
	}

	fmt.Fprint(w, line)
}

type PaletteMode int

const (
	PaletteMain PaletteMode = iota
	PaletteModels
	PaletteProviders
	PaletteConversations
	PaletteThemes
	PaletteAgents
	PaletteToolPicker
)

type commandPalette struct {
	list     list.Model
	mode     PaletteMode
	selected string
}

// fgAnsi returns a 24-bit ANSI foreground sequence for a #RRGGBB lipgloss color.
// Using \x1b[39m (fg-only reset) after dot glyphs instead of \x1b[0m (full reset)
// ensures any background set by PaginationStyle survives the entire row.
func fgAnsi(c lipgloss.Color) string {
	hex := strings.TrimPrefix(string(c), "#")
	if len(hex) != 6 {
		return ""
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

func newCommandPalette(title string, items []list.Item) commandPalette {
	l := list.New(items, itemDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.FilterInput.Placeholder = "Search"
	l.FilterInput.Prompt = "" // Use placeholder for the label or just leave it empty
	l.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	l.FilterInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	l.FilterInput.Cursor.Style = lipgloss.NewStyle().Background(ColorAccent).Foreground(lipgloss.Color("#000000"))

	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ColorAccent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ColorAccent)

	// Set dots directly on the paginator (Styles fields are only copied at New() time).
	// Use \x1b[39m (fg-only reset) instead of \x1b[0m (full reset) so the background
	// color applied by PaginationStyle is not cancelled mid-row.
	l.Paginator.ActiveDot   = fgAnsi(ColorAccent) + "●" + "\x1b[39m"
	l.Paginator.InactiveDot = fgAnsi(ColorMuted)  + "○" + "\x1b[39m"

	l.SetShowHelp(false)

	return commandPalette{
		list: l,
		mode: PaletteMain,
	}
}


