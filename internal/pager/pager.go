package pager

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	statusBarHeight = 1
	helpBoxPadding  = 2
	statusMsgDur    = 3 * time.Second
	logoText        = " godoc-cli "
)

var (
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#007d9c")).
			Bold(true)

	logoMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B6FFE4")).
			Background(lipgloss.Color("#1C8760")).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#2f2f2f", Dark: "#e6e6e6"}).
			Background(lipgloss.AdaptiveColor{Light: "#e6e6e6", Dark: "#303030"})

	statusBarMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#f8fff4", Dark: "#0b1a0f"}).
				Background(lipgloss.AdaptiveColor{Light: "#1c8760", Dark: "#1c8760"})

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#2f2f2f", Dark: "#d9d9d9"}).
			Background(lipgloss.AdaptiveColor{Light: "#f5f5f5", Dark: "#1e1e1e"}).
			Padding(1, 2)

	helpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1c8760", Dark: "#6fe7b3"}).
			Bold(true)
)

var helpEntries = []struct {
	keys string
	desc string
}{
	{"↑/k", "scroll up"},
	{"↓/j", "scroll down"},
	{"PgUp/b", "page up"},
	{"PgDn/f/space", "page down"},
	{"g/Home", "go to top"},
	{"G/End", "go to bottom"},
	{"d/Ctrl+D", "half page down"},
	{"u/Ctrl+U", "half page up"},
	{"c", "copy to clipboard"},
	{"?", "toggle help"},
	{"q/Esc", "quit"},
}

// Document represents the content and metadata to display in the pager.
type Document struct {
	Content string
	Raw     string
	Label   string
}

// Run launches an interactive pager to browse rendered markdown content.
func Run(doc Document) error {
	m := newModel(doc)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()

	return err
}

type statusMsgTimeoutMsg struct{}

type model struct {
	viewport      viewport.Model
	ready         bool
	width         int
	height        int
	doc           Document
	showHelp      bool
	statusMessage string
}

func newModel(doc Document) *model {
	vp := viewport.New(0, 0)
	vp.SetContent(doc.Content)
	vp.MouseWheelEnabled = true

	return &model{
		viewport: vp,
		doc:      doc,
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height
		m.setSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.showHelp {
				m.showHelp = false
				m.setSize(m.width, m.height)

				return m, nil
			}

			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			m.setSize(m.width, m.height)
		case "c":
			if m.doc.Raw != "" {
				termenv.Copy(m.doc.Raw)
				if err := clipboard.WriteAll(m.doc.Raw); err != nil {
					cmds = append(cmds, m.setStatusMessage(fmt.Sprintf("copy failed: %v", err)))
				} else {
					cmds = append(cmds, m.setStatusMessage("Copied contents"))
				}
			}
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "pgdown", "f", " ", "ctrl+f", "space":
			m.viewport.PageDown()
		case "pgup", "b", "ctrl+b":
			m.viewport.PageUp()
		case "ctrl+d", "d":
			m.viewport.HalfPageDown()
		case "ctrl+u", "u":
			m.viewport.HalfPageUp()
		case "g", "home":
			m.viewport.GotoTop()
		case "G", "end":
			m.viewport.GotoBottom()
		}

	case statusMsgTimeoutMsg:
		m.statusMessage = ""
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if !m.ready {
		return "Loading pager…"
	}

	var b strings.Builder

	b.WriteString(m.viewport.View())
	b.WriteRune('\n')
	b.WriteString(m.statusBar())
	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(m.helpView())
	}

	return b.String()
}

func (m *model) statusBar() string {
	width := m.viewport.Width
	if width <= 0 {
		width = lipgloss.Width(m.viewport.View())
	}

	percent := int(math.Round(math.Max(0, math.Min(1, m.viewport.ScrollPercent())) * 100))
	percentSegment := fmt.Sprintf(" %3d%% ", percent)
	helpLabel := " ? Help "
	if m.showHelp {
		helpLabel = " Close help "
	}

	rawLabel := strings.TrimSpace(m.doc.Label)
	if rawLabel == "" {
		rawLabel = "godoc-cli pager"
	}
	if m.statusMessage != "" {
		rawLabel = m.statusMessage
	}

	logoRendered := logoStyle.Render(logoText)
	statusStyle := statusBarStyle
	if m.statusMessage != "" {
		logoRendered = logoMsgStyle.Render(logoText)
		statusStyle = statusBarMsgStyle
	}

	availableWidth := max(width-lipgloss.Width(logoRendered), 0)
	headroom := lipgloss.Width(percentSegment + helpLabel)
	innerWidth := max(availableWidth-headroom-2, 0)
	labelInner := truncateMiddle(rawLabel, innerWidth)
	label := fmt.Sprintf(" %s ", labelInner)
	leftWidth := lipgloss.Width(label)
	spaceWidth := max(availableWidth-leftWidth-headroom, 0)
	statusContent := label + strings.Repeat(" ", spaceWidth) + percentSegment + helpLabel
	statusRendered := statusStyle.Render(statusContent)

	return logoRendered + statusRendered
}

func (m *model) helpHeight() int {
	return len(helpEntries) + helpBoxPadding
}

func (m *model) helpView() string {
	lines := make([]string, 0, len(helpEntries)+2)
	lines = append(lines, helpTitleStyle.Render("Controls"))
	lines = append(lines, "")
	for _, entry := range helpEntries {
		lines = append(lines, fmt.Sprintf("%-12s %s", entry.keys, entry.desc))
	}

	content := strings.Join(lines, "\n")

	return helpStyle.Width(maxInt(m.viewport.Width, lipgloss.Width(content))).Render(content)
}

func (m *model) setSize(width, height int) {
	m.viewport.Width = width

	contentHeight := height - statusBarHeight
	if m.showHelp {
		contentHeight -= m.helpHeight()
	}

	if contentHeight < 1 {
		contentHeight = 1
	}

	m.viewport.Height = contentHeight
}

func (m *model) setStatusMessage(msg string) tea.Cmd {
	m.statusMessage = msg

	return tea.Tick(statusMsgDur, func(time.Time) tea.Msg {
		return statusMsgTimeoutMsg{}
	})
}

func truncateMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}

	if utf8.RuneCountInString(s) <= max {
		return s
	}

	if max <= 1 {
		r, _ := utf8.DecodeRuneInString(s)

		return string(r)
	}

	ellipsis := "…"

	keep := max - utf8.RuneCountInString(ellipsis)
	if keep <= 1 {
		r, _ := utf8.DecodeRuneInString(s)
		return string(r)
	}

	front := keep / 2
	back := keep - front
	runes := []rune(s)

	return string(runes[:front]) + ellipsis + string(runes[len(runes)-back:])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
