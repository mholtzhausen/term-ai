package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	normalTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#DDDDDD")).
				PaddingLeft(3)

	selectedTitleStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("#AD58B4")).
				Foreground(lipgloss.Color("#AD58B4")).
				PaddingLeft(2)

	normalDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777777")).
			PaddingLeft(3)

	selectedDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8787AF")).
				PaddingLeft(2).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("#AD58B4"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3C3C3C")). // Just slightly lighter than default dark bg
			Italic(true)
)

type item struct {
	title, desc string
	hasActions  bool
	meta        map[string]interface{}
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 2 }
func (d itemDelegate) Spacing() int                              { return 1 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	title := i.title
	desc := i.desc
	isSelected := index == m.Index()

	var tStyle, dStyle lipgloss.Style
	if isSelected {
		tStyle = selectedTitleStyle
		dStyle = selectedDescStyle
	} else {
		tStyle = normalTitleStyle
		dStyle = normalDescStyle
	}

	// Render Title Line
	titleLine := tStyle.Render(title)

	// Add hints if selected and has actions
	if isSelected && i.hasActions {
		hint := "  e: update · d: delete "
		availWidth := m.Width() - lipgloss.Width(titleLine) - 2
		if availWidth > 20 {
			titleLine = lipgloss.JoinHorizontal(lipgloss.Top,
				titleLine,
				lipgloss.PlaceHorizontal(availWidth, lipgloss.Right, hintStyle.Render(hint)),
			)
		}
	}

	fmt.Fprintf(w, "%s\n%s", titleLine, dStyle.Render(desc))
}

type PaletteMode int

const (
	PaletteMain PaletteMode = iota
	PaletteModels
	PaletteProviders
	PaletteConversations
)

type commandPalette struct {
	list     list.Model
	mode     PaletteMode
	selected string
}

func newCommandPalette(title string, items []list.Item) commandPalette {
	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(lipgloss.Color("#5F5FD7")).Foreground(lipgloss.Color("#FFFFFF"))
	l.SetShowHelp(false)

	return commandPalette{
		list: l,
		mode: PaletteMain,
	}
}
