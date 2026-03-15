package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhai-org/mhai/internal/ai"
	"github.com/mhai-org/mhai/internal/config"
	"github.com/mhai-org/mhai/internal/db"
	"github.com/mhai-org/mhai/internal/persona"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5F5FD7")).
			Padding(0, 1).
			MarginRight(1)

	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#3C3C3C")).
			MarginBottom(1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8787AF"))

	footerStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("#3C3C3C")).
			PaddingTop(1)

	appStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5F5FD7"))
)

type model struct {
	program       *tea.Program
	viewport      viewport.Model
	textarea      textarea.Model
	persona       *persona.Persona
	provider      *config.Provider
	history       []ai.Message
	err           error
	loading       bool
	width         int
	height        int
	terminalWidth  int
	terminalHeight int
	currentOut     strings.Builder
	selectedModel  string
	palette        commandPalette
	showPalette    bool
	modalWidth     int
	modalHeight    int
	
	// Wizard
	showWizard   bool
	wizardStep   int
	wizardName   string
	wizardKey    string
	wizardUrl    string
}

type responseMsg string
type errMsg error
type chunkMsg string
type modelsLoadedMsg []string

func LaunchInteractive(p *persona.Persona, provider *config.Provider) error {
	m := initialModel(p, provider)
	p_tea := tea.NewProgram(&m, tea.WithAltScreen())
	m.program = p_tea
	_, err := p_tea.Run()
	return err
}

func initialModel(p *persona.Persona, provider *config.Provider) model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 10000
	ta.SetWidth(30)
	ta.SetHeight(1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(lipgloss.Color("#5F5FD7")).Foreground(lipgloss.Color("#FFFFFF"))
	l.SetShowHelp(false)

	vp := viewport.New(30, 10)
	vp.SetContent("Welcome to MHAI. How can I help you today?")

	m := model{
		textarea: ta,
		viewport: vp,
		persona:  p,
		provider: provider,
		selectedModel: "gpt-4",
		history:  []ai.Message{{Role: "system", Content: p.SystemPrompt}},
	}
	m.palette.list = l
	return m
}

func (m *model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	if m.showPalette {
		var lCmd tea.Cmd
		m.palette.list, lCmd = m.palette.list.Update(msg)
		tiCmd = lCmd
	} else if m.showWizard {
		m.textarea, tiCmd = m.textarea.Update(msg)
	} else {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlP:
			m.openMainPalette()
			return m, nil
		case tea.KeyEnter:
			if m.showPalette {
				return m.handlePaletteSelection()
			}
			if m.showWizard {
				return m.handleWizardNext()
			}
			if m.loading {
				return m, nil
			}
			input := m.textarea.Value()
			if strings.TrimSpace(input) == "" {
				return m, nil
			}

			m.history = append(m.history, ai.Message{Role: "user", Content: input})
			m.textarea.Reset()
			m.loading = true
			m.currentOut.Reset()
			
			m.updateViewport()

			return m, m.sendQuery(input)
		case tea.KeyEsc:
			if m.showPalette {
				if m.palette.mode != PaletteMain {
					m.openMainPalette()
					return m, nil
				}
				m.showPalette = false
				return m, nil
			}
			if m.showWizard {
				m.showWizard = false
				m.textarea.Reset()
				m.textarea.Placeholder = "Type your message here..."
				return m, nil
			}
			return m, tea.Quit
		}

	case chunkMsg:
		m.currentOut.WriteString(string(msg))
		m.updateViewport()
		return m, nil

	case modelsLoadedMsg:
		var items []list.Item
		for _, mName := range msg {
			items = append(items, item{title: mName, desc: "AI Model"})
		}
		m.palette = newCommandPalette("Select Model", items)
		m.palette.mode = PaletteModels
		m.palette.list.SetSize(m.modalWidth-4, m.modalHeight-4)
		return m, nil

	case responseMsg:
		m.history = append(m.history, ai.Message{Role: "assistant", Content: string(msg)})
		m.loading = false
		m.currentOut.Reset()
		m.updateViewport()
		return m, nil

	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		m.sizeApp()

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *model) sizeApp() {
	m.width = m.terminalWidth - 2
	m.height = m.terminalHeight - 2

	inputHeight := int(float64(m.height) * 0.3)
	if inputHeight < 3 {
		inputHeight = 3
	}
	
	headerH := 4
	footerH := inputHeight + 4 // Adjust for footer border + hint text
	
	m.viewport.Width = m.width
	m.viewport.Height = m.height - headerH - footerH
	
	m.textarea.SetWidth(m.width)
	m.textarea.SetHeight(inputHeight)

	m.modalWidth = int(float64(m.width) * 0.6)
	m.modalHeight = int(float64(m.height) * 0.6)
	if m.showPalette {
		m.palette.list.SetSize(m.modalWidth-4, m.modalHeight-4)
	}
}

func (m *model) openMainPalette() {
	items := []list.Item{
		item{title: "Select Model", desc: "Change the active LLM model"},
		item{title: "Select Provider", desc: "Switch between configured AI backends"},
	}
	m.palette = newCommandPalette("Command Palette", items)
	m.palette.mode = PaletteMain
	m.palette.list.SetSize(m.modalWidth-4, m.modalHeight-4)
	m.showPalette = true
}

func (m *model) handlePaletteSelection() (tea.Model, tea.Cmd) {
	i, ok := m.palette.list.SelectedItem().(item)
	if !ok {
		return m, nil
	}

	switch m.palette.mode {
	case PaletteMain:
		switch i.title {
		case "Select Model":
			return m.openModelsPalette()
		case "Select Provider":
			return m.openProvidersPalette()
		}
	case PaletteModels:
		m.selectedModel = i.title
		m.history = append(m.history, ai.Message{Role: "system", Content: fmt.Sprintf("Switched to model: %s", i.title)})
		m.showPalette = false
		m.updateViewport()
		return m, nil
	case PaletteProviders:
		if i.title == "Add New Provider..." {
			m.showPalette = false
			return m.startWizard()
		}
		d, _ := db.Connect()
		defer d.Conn.Close()
		p, err := config.GetProvider(d, i.title)
		if err == nil {
			m.provider = p
			m.history = append(m.history, ai.Message{Role: "system", Content: fmt.Sprintf("Switched to provider: %s", i.title)})
		}
		m.showPalette = false
		m.updateViewport()
		return m, nil
	}
	return m, nil
}

func (m *model) startWizard() (tea.Model, tea.Cmd) {
	m.showWizard = true
	m.wizardStep = 1
	m.textarea.Reset()
	m.textarea.Placeholder = "Enter provider name (e.g. anthropic)..."
	m.textarea.Focus()
	return m, nil
}

func (m *model) handleWizardNext() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.textarea.Value())
	if val == "" {
		return m, nil
	}

	switch m.wizardStep {
	case 1:
		m.wizardName = val
		m.wizardStep = 2
		m.textarea.Reset()
		m.textarea.Placeholder = "Enter API Key..."
	case 2:
		m.wizardKey = val
		m.wizardStep = 3
		m.textarea.Reset()
		m.textarea.Placeholder = "Enter API URL (e.g. https://api.anthropic.com/v1/messages)..."
	case 3:
		m.wizardUrl = val
		// Save to DB
		d, err := db.Connect()
		if err != nil {
			m.err = err
		} else {
			defer d.Conn.Close()
			err = config.SetProvider(d, m.wizardName, m.wizardKey, m.wizardUrl)
			if err != nil {
				m.err = err
			}
		}
		m.showWizard = false
		m.textarea.Reset()
		m.textarea.Placeholder = "Type your message here..."
		m.history = append(m.history, ai.Message{Role: "system", Content: fmt.Sprintf("Provider %s added successfully.", m.wizardName)})
		m.updateViewport()
	}
	return m, nil
}

func (m *model) openModelsPalette() (tea.Model, tea.Cmd) {
	m.palette = newCommandPalette("Loading Models...", []list.Item{item{title: "Loading...", desc: "Fetching from API"}})
	m.palette.mode = PaletteModels
	m.palette.list.SetSize(m.modalWidth-4, m.modalHeight-4)

	return m, func() tea.Msg {
		models, err := ai.ListModels(m.provider.ApiUrl, m.provider.ApiKey)
		if err != nil {
			return errMsg(err)
		}
		return modelsLoadedMsg(models)
	}
}

func (m *model) openProvidersPalette() (tea.Model, tea.Cmd) {
	d, _ := db.Connect()
	defer d.Conn.Close()
	providers, _ := config.ListProviders(d)

	var items []list.Item
	for _, p := range providers {
		items = append(items, item{title: p.Name, desc: p.ApiUrl})
	}
	items = append(items, item{title: "Add New Provider...", desc: "Setup a new AI backend"})
	m.palette = newCommandPalette("Select Provider", items)
	m.palette.mode = PaletteProviders
	m.palette.list.SetSize(m.modalWidth-4, m.modalHeight-4)
	return m, nil
}

func (m *model) updateViewport() {
	var b strings.Builder
	for _, msg := range m.history {
		if msg.Role == "system" {
			continue
		}
		
		role := "YOU"
		if msg.Role == "assistant" {
			role = "AI"
		}
		
		b.WriteString(fmt.Sprintf("**%s**:\n", role))
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}

	if m.currentOut.Len() > 0 {
		b.WriteString("**AI**:\n")
		b.WriteString(m.currentOut.String())
	}
	
	rendered, _ := glamour.Render(b.String(), "dark")
	m.viewport.SetContent(rendered)
	m.viewport.GotoBottom()
}

func (m *model) sendQuery(prompt string) tea.Cmd {
	return func() tea.Msg {
		var full strings.Builder
		
		writer := &tuiWriter{
			program: m.program,
			full:    &full,
		}

		err := ai.StreamChat(m.provider.ApiUrl, m.provider.ApiKey, m.selectedModel, m.persona.SystemPrompt, prompt, writer)
		if err != nil {
			return errMsg(err)
		}
		return responseMsg(full.String())
	}
}

type tuiWriter struct {
	program *tea.Program
	full    *strings.Builder
}

func (w *tuiWriter) Write(p []byte) (n int, err error) {
	w.program.Send(chunkMsg(string(p)))
	return w.full.Write(p)
}

func (m *model) View() string {
	if m.terminalWidth == 0 {
		return "Initializing full-screen TUI..."
	}

	cwd, _ := os.Getwd()
	t := time.Now()
	
	leftHeader := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("MHAI"),
		infoStyle.Render(fmt.Sprintf("📂 %s", cwd)),
	)
	// Right side of header
	rightHeader := lipgloss.JoinVertical(lipgloss.Right,
		infoStyle.Render(t.Format("Monday, Jan 02, 2006 | 15:04:05")),
		infoStyle.Render(fmt.Sprintf("Provider: %s | Model: %s", m.provider.Name, m.selectedModel)),
	)
	
	headerWidth := m.width
	header := headerStyle.Width(headerWidth).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			leftHeader,
			lipgloss.PlaceHorizontal(headerWidth-lipgloss.Width(leftHeader), lipgloss.Right, rightHeader),
		),
	)

	footer := footerStyle.Width(m.width).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.textarea.View(),
			"",
			infoStyle.Render(" Ctrl+P: Palette | Enter: Send | Esc: Exit"),
		),
	)

	ui := appStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.viewport.View(),
			footer,
		),
	)

	if m.showPalette {
		// Render Modal centered
		modal := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#5F5FD7")).
			Padding(1).
			Width(m.modalWidth).
			Height(m.modalHeight).
			Render(m.palette.list.View())

		// Center the modal over the UI
		ui = lipgloss.Place(m.terminalWidth, m.terminalHeight,
			lipgloss.Center, lipgloss.Center,
			modal,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#282828")),
		)
	}

	if m.showWizard {
		stepTitle := ""
		switch m.wizardStep {
		case 1: stepTitle = "1/3: Provider Name"
		case 2: stepTitle = "2/3: API Key"
		case 3: stepTitle = "3/3: API URL"
		}

		wizard := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#FFAF00")).
			Padding(1).
			Width(m.modalWidth).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				titleStyle.Background(lipgloss.Color("#FFAF00")).Render("Add Provider Wizard"),
				infoStyle.Render(stepTitle),
				"",
				m.textarea.View(),
				"",
				infoStyle.Render("Press Enter to continue, Esc to cancel"),
			))
		
		ui = lipgloss.Place(m.terminalWidth, m.terminalHeight,
			lipgloss.Center, lipgloss.Center,
			wizard,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#282828")),
		)
	}

	return ui
}
