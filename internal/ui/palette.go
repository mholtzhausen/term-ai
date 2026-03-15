package ui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#AD58B4"))
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type PaletteMode int

const (
	PaletteMain PaletteMode = iota
	PaletteModels
	PaletteProviders
)

type commandPalette struct {
	list     list.Model
	mode     PaletteMode
	selected string
}

func newCommandPalette(title string, items []list.Item) commandPalette {
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
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
