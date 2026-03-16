package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhai-org/term-ai/internal/agent"
	"github.com/mhai-org/term-ai/internal/ai"
	"github.com/mhai-org/term-ai/internal/config"
	"github.com/mhai-org/term-ai/internal/db"
	"github.com/mhai-org/term-ai/internal/memory"
	"github.com/mhai-org/term-ai/internal/tools"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#101010")).
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
)

type model struct {
	program       *tea.Program
	viewport      viewport.Model
	textarea      textarea.Model
	persona       *agent.Agent
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
	currentConversation *ai.Conversation

	// Theme
	theme         Theme
	previousTheme Theme

	// Wizard context
	wizardType         string // "provider" or "agent"
	wizardMode         string // "add" or "edit"
	showWizard         bool
	wizardStep         int
	wizardName         string
	wizardKey          string
	wizardUrl          string
	wizardPersonaPrompt string
	wizardAgentTools   string // existing tools for edit mode pre-population
	toolSelected       map[string]bool

	// In-process session memory shared across all turns and sub-agents.
	memory *memory.Memory

	// Prompt history (newest first)
	promptHistory []string
	historyIdx    int    // -1 = not browsing history
	historyDraft  string // saved live input while browsing
}

type responseMsg string
type errMsg error
type chunkMsg string
type modelsLoadedMsg []string

type toolCallMsg struct{ Name, Args string }
type toolResultMsg struct{ Name, Result string }
type agentResponseMsg struct {
	content string
	history []ai.Message
}

func LaunchInteractive(p *agent.Agent, provider *config.Provider, initialModelName string) error {
	activeTheme := DefaultTheme
	if d, dbErr := db.Connect(); dbErr == nil {
		if name, cfgErr := config.GetConfig(d, "active_theme"); cfgErr == nil {
			if t, ok := ThemesByName[name]; ok {
				activeTheme = t
			}
		}
		d.Conn.Close()
	}
	ApplyTheme(activeTheme)

	m := initialModel(p, provider, initialModelName)
	m.theme = activeTheme
	m.historyIdx = -1
	m.memory = memory.New()
	if d, dbErr := db.Connect(); dbErr == nil {
		if hist, hErr := loadPromptHistory(d); hErr == nil {
			m.promptHistory = hist
		}
		d.Conn.Close()
	}
	p_tea := tea.NewProgram(&m, tea.WithAltScreen())
	m.program = p_tea
	_, err := p_tea.Run()
	return err
}

func initialModel(p *agent.Agent, provider *config.Provider, initialModelName string) model {
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

	l := list.New([]list.Item{}, itemDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	vp := viewport.New(30, 10)
	vp.SetContent("Welcome to term-ai. How can I help you today?")

	m := model{
		textarea: ta,
		viewport: vp,
		persona:  p,
		provider: provider,
		selectedModel: initialModelName,
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
		// Handle custom keys BEFORE list update to prevent double-processing or state shifts
		if kmsg, ok := msg.(tea.KeyMsg); ok && m.palette.mode == PaletteProviders {
			// Don't trigger actions if typing in filter
			if m.palette.list.FilterState() != list.Filtering {
				switch kmsg.String() {
				case "d":
					// DELETE
					i, ok := m.palette.list.SelectedItem().(item)
					if ok && i.title != "Add New Provider..." {
						d, _ := db.Connect()
						if d != nil {
							config.DeleteProvider(d, i.title)
							d.Conn.Close()
						}
						m.openProvidersPalette()
						return m, nil
					}
				case "e":
					// EDIT
					i, ok := m.palette.list.SelectedItem().(item)
					if ok && i.title != "Add New Provider..." {
						d, _ := db.Connect()
						if d != nil {
							prov, err := config.GetProvider(d, i.title)
							d.Conn.Close()
							if err == nil {
								m.showPalette = false
								return m.startEditWizard(prov)
							}
						}
					}
				}
			}
		}

		if kmsg, ok := msg.(tea.KeyMsg); ok && m.palette.mode == PaletteToolPicker {
			if m.palette.list.FilterState() != list.Filtering && kmsg.String() == " " {
				if i, ok := m.palette.list.SelectedItem().(item); ok {
					if toolName, ok := i.meta["tool"].(string); ok {
						m.toolSelected[toolName] = !m.toolSelected[toolName]
						m.refreshToolPickerItems()
						return m, nil
					}
				}
			}
		}

		if kmsg, ok := msg.(tea.KeyMsg); ok && m.palette.mode == PaletteAgents {
			if m.palette.list.FilterState() != list.Filtering {
				switch kmsg.String() {
				case "d":
					i, ok := m.palette.list.SelectedItem().(item)
					if ok && i.title != "Add New Agent..." {
						d, _ := db.Connect()
						if d != nil {
							agent.UnsetAgent(d, i.title)
							d.Conn.Close()
						}
						m.openAgentsPalette()
						return m, nil
					}
				case "e":
					i, ok := m.palette.list.SelectedItem().(item)
					if ok && i.title != "Add New Agent..." {
						d, _ := db.Connect()
						if d != nil {
							a, err := agent.GetAgent(d, i.title)
							d.Conn.Close()
							if err == nil {
								m.showPalette = false
								return m.startEditAgentWizard(a)
							}
						}
					}
				}
			}
		}

		var lCmd tea.Cmd
		m.palette.list, lCmd = m.palette.list.Update(msg)
		tiCmd = lCmd

		// Live preview: apply theme as user navigates the theme picker
		if m.palette.mode == PaletteThemes {
			if hovered, ok := m.palette.list.SelectedItem().(item); ok {
				if t, ok := hovered.meta["theme"].(Theme); ok {
					ApplyTheme(t)
					m.theme = t
				}
			}
		}
	} else if m.showWizard {
		m.textarea, tiCmd = m.textarea.Update(msg)
	} else {
		// History navigation: intercept Up/Down before passing to textarea.
		if kmsg, ok := msg.(tea.KeyMsg); ok && !m.loading {
			if kmsg.Type == tea.KeyUp && m.textarea.Line() == 0 {
				if m.historyNavigateBack() {
					return m, nil
				}
			} else if kmsg.Type == tea.KeyDown && m.historyIdx >= 0 {
				m.historyNavigateForward()
				return m, nil
			}
		}
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlP:
			m.openMainPalette()
			return m, nil
		case tea.KeyCtrlA:
			return m.openAgentsPalette()
		case tea.KeyCtrlD:
			// Advance from multi-line system-prompt step in the agent wizard.
			if m.showWizard && m.wizardType == "agent" && m.wizardStep == 2 {
				return m.handleWizardNext()
			}
		case tea.KeyEnter:
			if m.showPalette {
				return m.handlePaletteSelection()
			}
			if m.showWizard {
				// For agent step 2 (multi-line system prompt) Enter inserts a
				// newline; the user presses Ctrl+D to advance instead.
				if m.wizardType == "agent" && m.wizardStep == 2 {
					break
				}
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
			m.historyIdx = -1
			m.historyDraft = ""
			// Prepend to in-memory list and persist to DB.
			m.promptHistory = append([]string{input}, m.promptHistory...)
			if len(m.promptHistory) > promptHistoryLimit {
				m.promptHistory = m.promptHistory[:promptHistoryLimit]
			}
			go func() {
				if d, err := db.Connect(); err == nil {
					_ = savePromptHistory(d, input)
					d.Conn.Close()
				}
			}()

			m.updateViewport()

			return m, m.sendQuery(input)
		case tea.KeyEsc:
			if m.showPalette {
				if m.palette.mode != PaletteMain {
					if m.palette.mode == PaletteThemes {
						ApplyTheme(m.previousTheme)
						m.theme = m.previousTheme
					}
					if m.palette.mode == PaletteToolPicker {
						m.toolSelected = nil
						m.wizardName = ""
						m.wizardPersonaPrompt = ""
					}
					m.openMainPalette()
					return m, nil
				}
				m.showPalette = false
				return m, nil
			}
			if m.showWizard {
				m.showWizard = false
				m.textarea.Reset()
				m.textarea.KeyMap.InsertNewline.SetEnabled(false)
				m.textarea.Placeholder = "Type your message here..."
				return m, nil
			}
			return m, tea.Quit
		}

	case chunkMsg:
		m.currentOut.WriteString(string(msg))
		m.updateViewport()
		return m, nil

	case toolCallMsg:
		m.currentOut.WriteString(fmt.Sprintf("\n🔧 **Calling** `%s`:\n```json\n%s\n```\n", msg.Name, msg.Args))
		m.updateViewport()
		return m, nil

	case toolResultMsg:
		result := msg.Result
		if len(result) > 500 {
			result = result[:500] + "\n... (truncated)"
		}
		m.currentOut.WriteString(fmt.Sprintf("\n📤 **Result** (`%s`):\n```\n%s\n```\n", msg.Name, result))
		m.updateViewport()
		return m, nil

	case agentResponseMsg:
		m.history = msg.history
		m.loading = false
		m.currentOut.Reset()
		// Save conversation
		d, _ := db.Connect()
		if d != nil {
			if m.currentConversation == nil {
				title := "TUI Conversation"
				if len(m.history) > 1 {
					title = strings.TrimSpace(m.history[1].Content)
					if len(title) > 30 {
						title = title[:27] + "..."
					}
				}
				m.currentConversation = &ai.Conversation{
					Title:        title,
					Platform:     "tui",
					ProviderName: m.provider.Name,
					ModelName:    m.selectedModel,
					PersonaName:  m.persona.Name,
				}
			}
			m.currentConversation.History = m.history
			ai.SaveConversation(d, m.currentConversation)
			d.Conn.Close()
		}
		m.updateViewport()
		return m, nil

	case modelsLoadedMsg:
		var items []list.Item
		for _, mName := range msg {
			items = append(items, item{title: mName, category: "Models", shortcut: "enter"})
		}
		m.palette = newCommandPalette("Models", items)
		m.palette.mode = PaletteModels
		m.applyPaletteSize()
		return m, nil

	case responseMsg:
		m.history = append(m.history, ai.Message{Role: "assistant", Content: string(msg)})
		m.loading = false
		m.currentOut.Reset()
		
		// Save conversation
		d, _ := db.Connect()
		if d != nil {
			if m.currentConversation == nil {
				title := "TUI Conversation"
				if len(m.history) > 1 {
					title = strings.TrimSpace(m.history[1].Content)
					if len(title) > 30 {
						title = title[:27] + "..."
					}
				}
				m.currentConversation = &ai.Conversation{
					Title:        title,
					Platform:     "tui",
					ProviderName: m.provider.Name,
					ModelName:    m.selectedModel,
					PersonaName:  m.persona.Name,
				}
			}
			m.currentConversation.History = m.history
			ai.SaveConversation(d, m.currentConversation)
			d.Conn.Close()
		}

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

// applyPaletteSize resizes the palette list and sets the pagination centering
// style in one call. Must be called after m.modalWidth/modalHeight are set.
func (m *model) applyPaletteSize() {
	w := m.modalWidth - 4
	m.palette.list.SetSize(w, m.modalHeight-4)
	m.palette.list.Styles.PaginationStyle = lipgloss.NewStyle().
		Width(w).
		Align(lipgloss.Center).
		Background(ColorBg)
}

func (m *model) sizeApp() {
	m.width = m.terminalWidth - 2
	m.height = m.terminalHeight - 2

	inputHeight := 3

	headerH := 4
	footerH := inputHeight + 4 // Adjust for footer border + hint text

	m.viewport.Width = m.width
	m.viewport.Height = m.height - headerH - footerH

	m.textarea.SetWidth(m.width)
	m.textarea.SetHeight(inputHeight)

	m.modalWidth = int(float64(m.width) * 0.6)
	m.modalHeight = int(float64(m.height) * 0.6)
	if m.showPalette {
		m.applyPaletteSize()
		switch m.palette.mode {
		case PaletteProviders, PaletteAgents:
			m.palette.list.AdditionalShortHelpKeys = func() []key.Binding {
				return []key.Binding{
					key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
					key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
				}
			}
		case PaletteToolPicker:
			m.palette.list.AdditionalShortHelpKeys = func() []key.Binding {
				return []key.Binding{
					key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
					key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "toggle/save")),
				}
			}
		}
	}
}

func (m *model) openMainPalette() {
	items := []list.Item{
		item{title: "Select Provider", category: "Suggested", shortcut: "ctrl+p"},
		item{title: "Select Model", category: "Suggested"},
		item{title: "Manage Agents", category: "Suggested", shortcut: "ctrl+a"},
		item{title: "Change Theme", category: "Appearance"},
		item{title: "Recent Conversations", category: "Session", shortcut: "ctrl+r"},
	}
	m.palette = newCommandPalette("Commands", items)
	m.palette.mode = PaletteMain
	m.applyPaletteSize()
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
		case "Manage Agents":
			return m.openAgentsPalette()
		case "Change Theme":
			return m.openThemesPalette()
		case "Recent Conversations":
			return m.openConversationsPalette()
		}
	case PaletteConversations:
		m.currentConversation = i.meta["conv"].(*ai.Conversation)
		m.history = m.currentConversation.History
		m.selectedModel = m.currentConversation.ModelName
		// Re-fetch provider if needed
		d, _ := db.Connect()
		if d != nil {
			p, err := config.GetProvider(d, m.currentConversation.ProviderName)
			if err == nil {
				m.provider = p
			}
			d.Conn.Close()
		}
		m.showPalette = false
		m.updateViewport()
		return m, nil
	case PaletteModels:
		m.selectedModel = i.title
		m.history = append(m.history, ai.Message{Role: "system", Content: fmt.Sprintf("Switched to model: %s", i.title)})
		m.showPalette = false
		
		// Persist change
		d, _ := db.Connect()
		if d != nil {
			config.SetConfig(d, "active_model", i.title)
			d.Conn.Close()
		}
		
		m.updateViewport()
		return m, nil
	case PaletteThemes:
		t, ok := i.meta["theme"].(Theme)
		if !ok {
			return m, nil
		}
		m.theme = t
		ApplyTheme(t)
		d, _ := db.Connect()
		if d != nil {
			config.SetConfig(d, "active_theme", t.Name)
			d.Conn.Close()
		}
		m.showPalette = false
		return m, nil
	case PaletteToolPicker:
		// Enter on an item toggles it; a dedicated "Save" item at the bottom confirms.
		if toolName, ok := i.meta["tool"].(string); ok {
			m.toolSelected[toolName] = !m.toolSelected[toolName]
			m.refreshToolPickerItems()
			return m, nil
		}
		if i.meta["save"] == true {
			return m.saveAgentFromPicker()
		}
		return m, nil
	case PaletteAgents:
		if i.title == "Add New Agent..." {
			m.showPalette = false
			return m.startAgentWizard()
		}
		d, _ := db.Connect()
		if d != nil {
			defer d.Conn.Close()
			a, err := agent.GetAgent(d, i.title)
			if err == nil {
				m.persona = a
				if len(m.history) > 0 {
					m.history[0] = ai.Message{Role: "system", Content: a.SystemPrompt}
				}
				config.SetConfig(d, "default_tui_agent", a.Name)
				m.history = append(m.history, ai.Message{Role: "system", Content: fmt.Sprintf("Switched to agent: %s", a.Name)})
			}
		}
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
			
			// Persist change
			config.SetConfig(d, "active_provider", i.title)
		}
		m.updateViewport()
		return m.openModelsPalette()
	}
	return m, nil
}

func (m *model) startWizard() (tea.Model, tea.Cmd) {
	m.showWizard = true
	m.wizardType = "provider"
	m.wizardMode = "add"
	m.wizardStep = 1
	m.textarea.Reset()
	m.textarea.Placeholder = "Enter provider name (e.g. anthropic)..."
	m.textarea.Focus()
	return m, nil
}

func (m *model) startEditWizard(p *config.Provider) (tea.Model, tea.Cmd) {
	m.showWizard = true
	m.wizardType = "provider"
	m.wizardMode = "edit"
	m.wizardStep = 2 // Skip name in edit mode
	m.wizardName = p.Name
	m.textarea.Reset()
	m.textarea.SetValue(p.ApiKey)
	m.textarea.Placeholder = "Enter new API Key..."
	m.textarea.Focus()
	return m, nil
}

func (m *model) handleWizardNext() (tea.Model, tea.Cmd) {
	if m.wizardType == "agent" {
		return m.handleAgentWizardNext()
	}
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
		msg := "added"
		if m.wizardMode == "edit" {
			msg = "updated"
		}
		m.history = append(m.history, ai.Message{Role: "system", Content: fmt.Sprintf("Provider %s %s successfully.", m.wizardName, msg)})
		m.updateViewport()
	}
	return m, nil
}

func (m *model) openModelsPalette() (tea.Model, tea.Cmd) {
	m.palette = newCommandPalette("Models", []list.Item{item{title: "Loading...", category: "Models"}})
	m.palette.mode = PaletteModels
	m.applyPaletteSize()

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
		items = append(items, item{title: p.Name, category: "Providers", hasActions: true, shortcut: "ctrl+x p"})
	}
	items = append(items, item{title: "Add New Provider...", category: "Actions", shortcut: "ctrl+n"})
	m.palette = newCommandPalette("Providers", items)
	m.palette.mode = PaletteProviders
	m.applyPaletteSize()
	m.palette.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		}
	}
	return m, nil
}

func (m *model) openAgentsPalette() (tea.Model, tea.Cmd) {
	d, _ := db.Connect()
	if d == nil {
		return m, nil
	}
	defer d.Conn.Close()
	agents, _ := agent.ListAgents(d)

	var items []list.Item
	for _, a := range agents {
		toolsStr := strings.Join(a.Tools, ", ")
		if toolsStr == "" {
			toolsStr = "no tools"
		}
		items = append(items, item{
			title:      a.Name,
			desc:       toolsStr,
			category:   "Agents",
			hasActions: true,
		})
	}
	items = append(items, item{title: "Add New Agent...", category: "Actions"})
	m.palette = newCommandPalette("Agents", items)
	m.palette.mode = PaletteAgents
	m.applyPaletteSize()
	m.showPalette = true
	m.palette.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		}
	}
	return m, nil
}

func (m *model) startAgentWizard() (tea.Model, tea.Cmd) {
	m.showWizard = true
	m.wizardType = "agent"
	m.wizardMode = "add"
	m.wizardStep = 1
	m.wizardAgentTools = ""
	m.textarea.Reset()
	m.textarea.Placeholder = "Enter agent name (e.g. researcher)..."
	m.textarea.Focus()
	return m, nil
}

func (m *model) startEditAgentWizard(a *agent.Agent) (tea.Model, tea.Cmd) {
	m.showWizard = true
	m.wizardType = "agent"
	m.wizardMode = "edit"
	m.wizardStep = 2
	m.wizardName = a.Name
	m.wizardAgentTools = strings.Join(a.Tools, ", ")
	m.textarea.Reset()
	m.textarea.SetValue(a.SystemPrompt)
	m.textarea.KeyMap.InsertNewline.SetEnabled(true)
	m.textarea.Placeholder = "Edit system prompt... (Ctrl+D to continue)"
	m.textarea.Focus()
	return m, nil
}

func (m *model) handleAgentWizardNext() (tea.Model, tea.Cmd) {
	switch m.wizardStep {
	case 1:
		val := strings.TrimSpace(m.textarea.Value())
		if val == "" {
			return m, nil
		}
		m.wizardName = val
		m.wizardStep = 2
		m.textarea.Reset()
		m.textarea.KeyMap.InsertNewline.SetEnabled(true)
		m.textarea.Placeholder = "Enter system prompt... (Ctrl+D to continue)"
	case 2:
		m.wizardPersonaPrompt = m.textarea.Value()
		m.textarea.Reset()
		m.textarea.KeyMap.InsertNewline.SetEnabled(false)
		m.textarea.Placeholder = "Type your message here..."
		return m.openToolPickerPalette()
	}
	return m, nil
}

func (m *model) openToolPickerPalette() (tea.Model, tea.Cmd) {
	// Seed checked state from wizardAgentTools (edit mode pre-population).
	m.toolSelected = make(map[string]bool)
	for _, t := range tools.ParseTools(m.wizardAgentTools) {
		m.toolSelected[t] = true
	}
	items := m.buildToolPickerItems()
	m.palette = newCommandPalette("Select Tools", items)
	m.palette.mode = PaletteToolPicker
	m.applyPaletteSize()
	m.showPalette = true
	m.showWizard = false
	return m, nil
}

func (m *model) buildToolPickerItems() []list.Item {
	var items []list.Item
	for _, name := range tools.Available() {
		check := "[ ]"
		if m.toolSelected[name] {
			check = "[✓]"
		}
		items = append(items, item{
			title:    fmt.Sprintf("%s %s", check, name),
			category: "Tools",
			meta:     map[string]interface{}{"tool": name},
		})
	}
	// Confirm button at the bottom.
	items = append(items, item{
		title:    "── Save Agent ──",
		category: "Actions",
		meta:     map[string]interface{}{"save": true},
	})
	return items
}

func (m *model) refreshToolPickerItems() {
	// Preserve the cursor position while rebuilding checked state.
	cursor := m.palette.list.Index()
	m.palette.list.SetItems(m.buildToolPickerItems())
	m.palette.list.Select(cursor)
}

func (m *model) saveAgentFromPicker() (tea.Model, tea.Cmd) {
	var selected []string
	for _, name := range tools.Available() {
		if m.toolSelected[name] {
			selected = append(selected, name)
		}
	}
	d, err := db.Connect()
	if err != nil {
		m.err = err
		return m, nil
	}
	defer d.Conn.Close()
	if err := agent.SetAgent(d, m.wizardName, m.wizardPersonaPrompt, selected); err != nil {
		m.err = err
		return m, nil
	}
	action := "created"
	if m.wizardMode == "edit" {
		action = "updated"
	}
	m.history = append(m.history, ai.Message{
		Role:    "system",
		Content: fmt.Sprintf("Agent %q %s.", m.wizardName, action),
	})
	m.toolSelected = nil
	m.showPalette = false
	m.updateViewport()
	return m, nil
}

// historyNavigateBack moves one step older in prompt history.
// Returns true if it handled the key (caller should skip textarea update).
func (m *model) historyNavigateBack() bool {
	if len(m.promptHistory) == 0 {
		return false
	}
	if m.historyIdx == -1 {
		// First press: save whatever is currently typed.
		m.historyDraft = m.textarea.Value()
		m.historyIdx = 0
	} else if m.historyIdx < len(m.promptHistory)-1 {
		m.historyIdx++
	} else {
		return false // already at oldest entry
	}
	m.textarea.SetValue(m.promptHistory[m.historyIdx])
	return true
}

// historyNavigateForward moves one step newer in prompt history.
func (m *model) historyNavigateForward() {
	if m.historyIdx <= 0 {
		// Back to the live draft.
		m.historyIdx = -1
		m.textarea.SetValue(m.historyDraft)
		m.historyDraft = ""
		return
	}
	m.historyIdx--
	m.textarea.SetValue(m.promptHistory[m.historyIdx])
}

func (m *model) updateViewport() {
	var b strings.Builder
	for _, msg := range m.history {
		switch msg.Role {
		case "system":
			continue
		case "user":
			b.WriteString(fmt.Sprintf("**YOU**:\n%s\n\n", msg.Content))
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					b.WriteString(fmt.Sprintf("🔧 **Calling** `%s`:\n```json\n%s\n```\n\n", tc.Function.Name, tc.Function.Arguments))
				}
			} else {
				b.WriteString(fmt.Sprintf("**AI**:\n%s\n\n", msg.Content))
			}
		case "tool":
			result := msg.Content
			if len(result) > 500 {
				result = result[:500] + "\n... (truncated)"
			}
			b.WriteString(fmt.Sprintf("📤 **Result** (`%s`):\n```\n%s\n```\n\n", msg.Name, result))
		}
	}

	if m.currentOut.Len() > 0 {
		b.WriteString("**AI**:\n")
		b.WriteString(m.currentOut.String())
	}
	
	glamourStyle := m.theme.GlamourStyle
	if glamourStyle == "" {
		glamourStyle = "dark"
	}
	rendered, _ := glamour.Render(b.String(), glamourStyle)
	m.viewport.SetContent(rendered)
	m.viewport.GotoBottom()
}

func (m *model) sendQuery(prompt string) tea.Cmd {
	// Capture values needed in the goroutine.
	provider := m.provider
	model := m.selectedModel
	history := m.history
	persona := m.persona
	mem := m.memory

	return func() tea.Msg {
		var full strings.Builder

		if len(persona.Tools) > 0 {
			r := &agent.Runner{
				ApiUrl: provider.ApiUrl,
				ApiKey: provider.ApiKey,
				Model:  model,
				Memory: mem,
				OnToolCall: func(name, args string) {
					m.program.Send(toolCallMsg{Name: name, Args: args})
				},
				OnToolResult: func(name, result string) {
					m.program.Send(toolResultMsg{Name: name, Result: result})
				},
			}
			updatedHistory, err := r.Run(history, persona.Tools, &full)
			if err != nil {
				history = append(history, ai.Message{Role: "system", Content: "[Agent Error] " + err.Error()})
				return agentResponseMsg{content: full.String(), history: history}
			}
			return agentResponseMsg{content: full.String(), history: updatedHistory}
		}

		writer := &tuiWriter{program: m.program, full: &full}
		err := ai.StreamChatWithHistory(provider.ApiUrl, provider.ApiKey, model, history, writer)
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
		titleStyle.Render("term-ai"),
		infoStyle.Render(fmt.Sprintf("📂 %s", cwd)),
	)
	// Right side of header
	rightHeader := lipgloss.JoinVertical(lipgloss.Right,
		infoStyle.Render(t.Format("Monday, Jan 02, 2006 | 15:04:05")),
		infoStyle.Render(fmt.Sprintf("Agent: %s (%d tools) | Provider: %s | Model: %s",
			m.persona.Name, len(m.persona.Tools), m.provider.Name, m.selectedModel)),
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
			infoStyle.Render(" Ctrl+P: Palette | Ctrl+A: Agents | Enter: Send | Esc: Exit"),
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
		paletteTitle := "Commands"
		paletteHint := "esc"
		if m.palette.mode == PaletteToolPicker {
			paletteTitle = "Select Tools  (3/3)"
			paletteHint = "space: toggle  ·  enter: toggle/save  ·  esc: cancel"
		}
		// Custom header
		headerLeft := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(paletteTitle)
		headerRight := lipgloss.NewStyle().Foreground(ColorMuted).Render(paletteHint)
		header := lipgloss.JoinHorizontal(lipgloss.Top,
			headerLeft,
			lipgloss.PlaceHorizontal(m.modalWidth-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight)-4, lipgloss.Right, headerRight),
		)

		// Render Modal centered
		modal := lipgloss.NewStyle().
			Background(ColorBg).
			Padding(1, 2).
			Width(m.modalWidth).
			Height(m.modalHeight).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				header,
				"",
				m.palette.list.View(),
			))

		// Center the modal over the UI with a dimmed background.
		ui = placeOverlayCenter(ui, modal, m.terminalWidth, m.terminalHeight)
	}

	if m.showWizard {
		var wizardTitle, stepTitle, hintText string

		if m.wizardType == "agent" {
			wizardTitle = "Add Agent Wizard"
			if m.wizardMode == "edit" {
				wizardTitle = "Edit Agent Wizard"
			}
			switch m.wizardStep {
			case 1:
				stepTitle = "1/2: Agent Name"
				hintText = "Press Enter to continue, Esc to cancel"
			case 2:
				stepTitle = "2/2: System Prompt"
				hintText = "Press Ctrl+D to continue, Esc to cancel"
			}
		} else {
			wizardTitle = "Add Provider Wizard"
			if m.wizardMode == "edit" {
				wizardTitle = "Edit Provider Wizard"
			}
			switch m.wizardStep {
			case 1:
				stepTitle = "1/3: Provider Name"
			case 2:
				stepTitle = "2/3: API Key"
			case 3:
				stepTitle = "3/3: API URL"
			}
			hintText = "Press Enter to continue, Esc to cancel"
		}

		wizard := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorAccent).
			Padding(1).
			Width(m.modalWidth).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				titleStyle.Background(ColorAccent).Render(wizardTitle),
				infoStyle.Render(stepTitle),
				"",
				m.textarea.View(),
				"",
				infoStyle.Render(hintText),
			))

		ui = placeOverlayCenter(ui, wizard, m.terminalWidth, m.terminalHeight)
	}

	return ui
}
func (m *model) openThemesPalette() (tea.Model, tea.Cmd) {
	m.previousTheme = m.theme
	var items []list.Item
	for _, t := range BuiltInThemes {
		title := t.Name
		if t.Name == m.theme.Name {
			title += " ✓"
		}
		items = append(items, item{
			title:    title,
			category: "Themes",
			meta:     map[string]interface{}{"theme": t},
		})
	}
	m.palette = newCommandPalette("Themes", items)
	m.palette.mode = PaletteThemes
	m.applyPaletteSize()
	m.showPalette = true
	return m, nil
}

func (m *model) openConversationsPalette() (tea.Model, tea.Cmd) {
	d, _ := db.Connect()
	if d == nil {
		return m, nil
	}
	defer d.Conn.Close()
	convs, _ := ai.ListConversations(d)

	var items []list.Item
	for _, c := range convs {
		items = append(items, item{
			title:    c.Title,
			category: "Conversations",
			shortcut: c.UpdatedAt.Format("Jan 02"),
			meta:     map[string]interface{}{"conv": &c},
		})
	}
	
	m.palette = newCommandPalette("History", items)
	m.palette.mode = PaletteConversations
	m.applyPaletteSize()
	return m, nil
}
