package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type statusModel struct {
	spinner       spinner.Model
	tokens        int
	contextTokens int
	start         time.Time
	done          bool
	resuming      string
}

type ResumeMsg string

type TokenMsg int

func (m statusModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case TokenMsg:
		m.tokens += int(msg)
		m.contextTokens += int(msg)
		return m, nil
	case bool:
		if msg {
			m.done = true
			return m, tea.Quit
		}
	case ResumeMsg:
		m.resuming = string(msg)
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m statusModel) View() string {
	if m.done {
		return ""
	}
	elapsed := time.Since(m.start).Seconds()
	tps := 0.0
	if elapsed > 0.2 {
		tps = float64(m.tokens) / elapsed
	}

	ctxInfo := lipgloss.NewStyle().Foreground(StatusTokensColor).Bold(true).Render(formatTokens(m.contextTokens))
	ctxRight := lipgloss.NewStyle().Foreground(StatusLabelColor).Render("ctx: ") + ctxInfo

	left := fmt.Sprintf("%s %s %s tokens | %s tokens/sec",
		m.spinner.View(),
		lipgloss.NewStyle().Foreground(StatusLabelColor).Render("Generating..."),
		lipgloss.NewStyle().Foreground(StatusTokensColor).Bold(true).Render(fmt.Sprintf("%d", m.tokens)),
		lipgloss.NewStyle().Foreground(StatusTpsColor).Render(fmt.Sprintf("%.1f", tps)),
	)

	termWidth, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || termWidth <= 0 {
		termWidth = 80
	}
	pad := termWidth - lipgloss.Width(left) - lipgloss.Width(ctxRight)
	if pad < 1 {
		pad = 1
	}
	status := left + strings.Repeat(" ", pad) + ctxRight

	if m.resuming != "" {
		status = lipgloss.NewStyle().Foreground(StatusTokensColor).Italic(true).Render(m.resuming) + "\n" + status
	}
	return status
}

func RunStatusProgram(resumeMsg string, contextTokens int) (*tea.Program, chan int, chan bool) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#AD58B4"))

	m := statusModel{
		spinner:       s,
		start:         time.Now(),
		resuming:      resumeMsg,
		contextTokens: contextTokens,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	tokenChan := make(chan int, 100)
	doneChan := make(chan bool)

	go func() {
		for {
			select {
			case count := <-tokenChan:
				p.Send(TokenMsg(count))
			case <-doneChan:
				p.Send(true)
				return
			}
		}
	}()

	return p, tokenChan, doneChan
}

type TokenCounterWriter struct {
	Writer    io.Writer
	TokenChan chan int
}

func (w *TokenCounterWriter) Write(p []byte) (n int, err error) {
	// Approx tokens: length / 4
	tokens := len(string(p)) / 4
	if tokens == 0 && len(p) > 0 {
		tokens = 1
	}
	w.TokenChan <- tokens
	return w.Writer.Write(p)
}
