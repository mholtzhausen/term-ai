package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds the full color palette and style settings for one visual theme.
type Theme struct {
	Name        string
	Description string
	Dark        bool

	// Background layers
	Bg      lipgloss.Color
	Surface lipgloss.Color

	// Text
	Text      lipgloss.Color
	TextMuted lipgloss.Color

	// Accents
	Accent   lipgloss.Color
	AccentFg lipgloss.Color // foreground rendered on top of Accent background
	Header   lipgloss.Color

	// Structure
	Border lipgloss.Color

	// Status bar colors consumed by direct.go
	StatusLabel  lipgloss.Color
	StatusTokens lipgloss.Color
	StatusTps    lipgloss.Color

	// GlamourStyle is passed to glamour.Render ("dark", "light", "dracula", "tokyo-night", …)
	GlamourStyle string
}

// Status bar color vars — read by direct.go on every render; kept in sync by ApplyTheme.
var (
	StatusLabelColor  = lipgloss.Color("#8787AF")
	StatusTokensColor = lipgloss.Color("#AD58B4")
	StatusTpsColor    = lipgloss.Color("#5F5FD7")
)

// BuiltInThemes is the ordered list of all bundled themes shown in the picker.
var BuiltInThemes = []Theme{
	{
		Name: "term-ai Default", Description: "Warm peach on near-black — the original look",
		Dark: true, Bg: "#0C0C0C", Surface: "#242424", Text: "#E0E0E0", TextMuted: "#707070",
		Accent: "#FFB38A", AccentFg: "#101010", Header: "#8B89E1", Border: "#3C3C3C",
		StatusLabel: "#8787AF", StatusTokens: "#AD58B4", StatusTps: "#5F5FD7",
		GlamourStyle: "dark",
	},
	{
		Name: "Dracula", Description: "Classic purple night with high contrast",
		Dark: true, Bg: "#282A36", Surface: "#44475A", Text: "#F8F8F2", TextMuted: "#6272A4",
		Accent: "#FF79C6", AccentFg: "#282A36", Header: "#BD93F9", Border: "#6272A4",
		StatusLabel: "#6272A4", StatusTokens: "#BD93F9", StatusTps: "#50FA7B",
		GlamourStyle: "dracula",
	},
	{
		Name: "Nord", Description: "Arctic calm — muted blues and greys",
		Dark: true, Bg: "#2E3440", Surface: "#3B4252", Text: "#ECEFF4", TextMuted: "#616E88",
		Accent: "#88C0D0", AccentFg: "#2E3440", Header: "#81A1C1", Border: "#4C566A",
		StatusLabel: "#616E88", StatusTokens: "#88C0D0", StatusTps: "#A3BE8C",
		GlamourStyle: "dark",
	},
	{
		Name: "Tokyo Night", Description: "Neon city — deep blue with vivid highlights",
		Dark: true, Bg: "#1A1B2E", Surface: "#24283B", Text: "#C0CAF5", TextMuted: "#565F89",
		Accent: "#F7768E", AccentFg: "#1A1B2E", Header: "#7AA2F7", Border: "#3D4166",
		StatusLabel: "#565F89", StatusTokens: "#7AA2F7", StatusTps: "#9ECE6A",
		GlamourStyle: "tokyo-night",
	},
	{
		Name: "Gruvbox Dark", Description: "Warm retro vibes with earthy amber tones",
		Dark: true, Bg: "#282828", Surface: "#3C3836", Text: "#EBDBB2", TextMuted: "#928374",
		Accent: "#FABD2F", AccentFg: "#282828", Header: "#B8BB26", Border: "#504945",
		StatusLabel: "#928374", StatusTokens: "#FABD2F", StatusTps: "#B8BB26",
		GlamourStyle: "dark",
	},
	{
		Name: "Catppuccin Mocha", Description: "Soft lavender dark — pastel elegance",
		Dark: true, Bg: "#1E1E2E", Surface: "#313244", Text: "#CDD6F4", TextMuted: "#6C7086",
		Accent: "#CBA6F7", AccentFg: "#1E1E2E", Header: "#89B4FA", Border: "#45475A",
		StatusLabel: "#6C7086", StatusTokens: "#CBA6F7", StatusTps: "#89B4FA",
		GlamourStyle: "dark",
	},
	{
		Name: "One Dark", Description: "Atom editor's classic dark palette",
		Dark: true, Bg: "#282C34", Surface: "#31353F", Text: "#ABB2BF", TextMuted: "#5C6370",
		Accent: "#E06C75", AccentFg: "#282C34", Header: "#61AFEF", Border: "#3E4451",
		StatusLabel: "#5C6370", StatusTokens: "#E06C75", StatusTps: "#98C379",
		GlamourStyle: "dark",
	},
	{
		Name: "Monokai", Description: "Vivid black — the Sublime Text classic",
		Dark: true, Bg: "#272822", Surface: "#3E3D32", Text: "#F8F8F2", TextMuted: "#75715E",
		Accent: "#A6E22E", AccentFg: "#272822", Header: "#FD971F", Border: "#49483E",
		StatusLabel: "#75715E", StatusTokens: "#A6E22E", StatusTps: "#66D9EF",
		GlamourStyle: "dark",
	},
	{
		Name: "Rosé Pine", Description: "Dusty rose on deep plum — warm and quiet",
		Dark: true, Bg: "#191724", Surface: "#1F1D2E", Text: "#E0DEF4", TextMuted: "#6E6A86",
		Accent: "#EBBCBA", AccentFg: "#191724", Header: "#C4A7E7", Border: "#26233A",
		StatusLabel: "#6E6A86", StatusTokens: "#EBBCBA", StatusTps: "#9CCFD8",
		GlamourStyle: "dark",
	},
	{
		Name: "Solarized Light", Description: "The classic light theme — easy on the eyes",
		Dark: false, Bg: "#FDF6E3", Surface: "#EEE8D5", Text: "#657B83", TextMuted: "#93A1A1",
		Accent: "#268BD2", AccentFg: "#FDF6E3", Header: "#2AA198", Border: "#D3CBB8",
		StatusLabel: "#93A1A1", StatusTokens: "#268BD2", StatusTps: "#2AA198",
		GlamourStyle: "light",
	},
}

// ThemesByName enables O(1) lookup by name.
var ThemesByName map[string]Theme

// DefaultTheme is applied when no theme is stored in config.
var DefaultTheme = BuiltInThemes[0]

func init() {
	ThemesByName = make(map[string]Theme, len(BuiltInThemes))
	for _, t := range BuiltInThemes {
		ThemesByName[t.Name] = t
	}
}

// ApplyTheme updates all package-level color and style vars to reflect t.
// Call this at TUI startup and whenever the user selects a new theme.
// Safe to call from the Bubble Tea update loop because Bubble Tea is single-threaded.
func ApplyTheme(t Theme) {
	// -- Shared color vars (palette.go) --
	ColorBg = t.Bg
	ColorSurface = t.Surface
	ColorText = t.Text
	ColorMuted = t.TextMuted
	ColorAccent = t.Accent
	ColorHeader = t.Header
	ColorBorder = t.Border

	// -- Status bar color vars (direct.go) --
	StatusLabelColor = t.StatusLabel
	StatusTokensColor = t.StatusTokens
	StatusTpsColor = t.StatusTps

	// -- Rebuild palette.go styles --
	normalTitleStyle = lipgloss.NewStyle().
		Foreground(ColorText).
		PaddingLeft(3)

	selectedTitleStyle = lipgloss.NewStyle().
		Background(ColorAccent).
		Foreground(t.AccentFg).
		PaddingLeft(3).
		Bold(true)

	normalDescStyle = lipgloss.NewStyle().
		Foreground(ColorMuted).
		PaddingLeft(3)

	selectedDescStyle = lipgloss.NewStyle().
		Background(ColorAccent).
		Foreground(t.AccentFg).
		PaddingLeft(3)

	shortcutStyle = lipgloss.NewStyle().
		Foreground(ColorMuted).
		PaddingRight(3).
		Align(lipgloss.Right)

	selectedShortcutStyle = lipgloss.NewStyle().
		Background(ColorAccent).
		Foreground(t.AccentFg).
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

	// -- Rebuild interactive.go styles --
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AccentFg).
		Background(ColorAccent).
		Padding(0, 1).
		MarginRight(1)

	headerStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		MarginBottom(1)

	infoStyle = lipgloss.NewStyle().
		Foreground(ColorMuted)

	footerStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		PaddingTop(1)

	appStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder)
}
