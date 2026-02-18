package pager

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	statusBarHeight   = 1
	searchBoxHeight   = 1
	helpBoxPadding    = 2
	statusMsgDur      = 3 * time.Second
	searchDebounceDur = 100 * time.Millisecond
	logoText          = " godoc-cli "
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

	selectedResultStyle = lipgloss.NewStyle().Background(lipgloss.Color("#00FF00"))
	resultStyle         = lipgloss.NewStyle().Background(lipgloss.Color("#FF00FF"))
	noResultsStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#b22222"))
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

type searchResult struct {
	Line  int
	Index int
}

type searchMsg string

type model struct {
	viewport           viewport.Model
	textarea           textarea.Model
	ready              bool
	width              int
	height             int
	doc                Document
	showHelp           bool
	statusMessage      string
	isSearching        bool
	searchResults      []searchResult
	currentResultIndex int
	navigationMode     bool
}

func newModel(doc Document) *model {
	vp := viewport.New(0, 0)
	vp.SetContent(doc.Content)
	vp.MouseWheelEnabled = true

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "/"

	return &model{
		viewport: vp,
		doc:      doc,
		textarea: ta,
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

	case searchMsg:
		if m.textarea.Value() == string(msg) {
			m.highlightMatches()
			m.navigateToNextResult()
			return m, nil
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.isSearching && !m.navigationMode {
			var cmd tea.Cmd
			switch msg.String() {
			case "esc":
				m.handleDeactivations()
				m.setSize(m.width, m.height)
				return m, nil
			case "enter":
				return m, m.handleNavigationActivation()
			}

			m.textarea, cmd = m.textarea.Update(msg)
			currentValue := m.textarea.Value()
			return m, tea.Batch(cmd, func() tea.Msg {
				time.Sleep(searchDebounceDur)
				return searchMsg(currentValue)
			})
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.showHelp {
				m.showHelp = false
				m.setSize(m.width, m.height)
				return m, nil
			}

			if m.navigationMode {
				m.handleDeactivations()
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
		case "/":
			m.resetSearchResults()
			m.textarea.Reset()
			if m.navigationMode {
				m.handleDeactivations()
				return m, nil
			}
			m.setSize(m.width, m.height) // force layout recalculation
			return m, m.handleSearchActivation(msg)
		case "enter":
			return m, m.handleNavigationActivation()
		case "n":
			return m, m.handleNavigationForward(msg)
		case "N":
			return m, m.handleNavigationBackwards(msg)
		}

	case statusMsgTimeoutMsg:
		m.statusMessage = ""
	}

	var cmd tea.Cmd

	if m.isSearching {
		cmd = m.updateTextArea(msg)
		m.setSize(m.width, m.height)
		m.highlightMatches()
	}

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

	top := m.viewport.View()

	searchRow := ""
	if m.isSearching {
		counter := fmt.Sprintf("%d/%d", m.currentResultIndex+1, len(m.searchResults))
		if !m.hasSearchResults() {
			counter = noResultsStyle.Render("0/0")
		}
		counterStyled := lipgloss.NewStyle().
			Align(lipgloss.Right).
			Render(counter)

		searchRow = lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.textarea.View(),
			counterStyled,
		)
	}

	bottom := m.statusBar()
	if m.showHelp {
		bottom = lipgloss.JoinVertical(
			lipgloss.Left,
			bottom,
			m.helpView(),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		top,
		searchRow,
		bottom,
	)
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

func (m *model) updateTextArea(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return cmd
}

func (m *model) highlightMatches() {
	searchQuery := m.textarea.Value()
	if searchQuery == "" {
		return
	}

	m.resetSearchResults()
	m.findAndHighlightMatches(searchQuery)
}

func (m *model) resetSearchResults() {
	m.searchResults = []searchResult{}
}

func (m *model) findAndHighlightMatches(searchQuery string) {
	lines := strings.Split(m.doc.Content, "\n")
	var processedLines []string
	for i, line := range lines {
		processedLines = append(processedLines, m.processLineForCaseInsensitiveMatches(i, line, searchQuery))
	}
	m.viewport.SetContent(strings.Join(processedLines, "\n"))
}

func (m *model) processLineForCaseInsensitiveMatches(lineIndex int, line, searchQuery string) string {
	var highlightedLine string
	var startPos int

	lowercaseline := strings.ToLower(line)
	lowercasesearchQuery := strings.ToLower(searchQuery)

	for {
		index := strings.Index(lowercaseline[startPos:], lowercasesearchQuery)
		if index < 0 {
			highlightedLine += line[startPos:]
			break
		}

		m.storeSearchResult(lineIndex, startPos+index)
		highlightedLine += m.highlightMatch(lineIndex, startPos, index, lowercasesearchQuery, line)
		startPos += index + len(lowercasesearchQuery)
	}

	return highlightedLine
}

func (m *model) highlightMatch(lineIndex, startPos, index int, searchQuery, line string) string {
	styleToUse := m.setHighlightStyle(lineIndex, startPos+index)
	matchedPart := line[startPos+index : startPos+index+len(searchQuery)]
	return line[startPos:startPos+index] + styleToUse.Render(matchedPart)
}

func (m *model) storeSearchResult(line, index int) {
	m.searchResults = append(m.searchResults, searchResult{Line: line, Index: index})
}

func (m *model) setHighlightStyle(lineIndex, index int) lipgloss.Style {
	if m.currentResultIndex >= 0 && m.currentResultIndex < len(m.searchResults) {
		if lineIndex == m.searchResults[m.currentResultIndex].Line && index == m.searchResults[m.currentResultIndex].Index {
			return selectedResultStyle
		}
	}
	return resultStyle
}

func (m *model) setShowSearch(v bool) {
	m.isSearching = v
	if v {
		m.textarea.Focus()
	}
	m.setSize(m.width, m.height)
}

func (m *model) handleDeactivations() {
	if m.navigationMode {
		m.navigationMode = false
		m.textarea.Focus()
	}
	if m.isSearching {
		m.setShowSearch(false)
		m.viewport.SetContent(m.doc.Content)
		m.setSize(m.width, m.height)
	}
}

func (m *model) hasSearchResults() bool {
	return len(m.searchResults) > 0
}

func (m *model) incrementSearchIndex() {
	m.currentResultIndex = (m.currentResultIndex + 1) % len(m.searchResults)
}

func (m *model) decrementSearchIndex() {
	m.currentResultIndex = m.currentResultIndex - 1
	if m.currentResultIndex < 0 {
		m.currentResultIndex = len(m.searchResults) - 1
	}
}

func (m *model) scrollViewportToLine(line int) {
	// Check if the resultLine is currently visible
	topLine := m.viewport.YOffset
	bottomLine := topLine + m.viewport.Height - 1 // -1 because it's zero-based index
	for line < topLine || line > bottomLine {
		if line < topLine {
			m.viewport.ViewUp()
		} else {
			m.viewport.ViewDown()
		}
		// Update topLine and bottomLine after scrolling
		topLine = m.viewport.YOffset
		bottomLine = topLine + m.viewport.Height - 1
	}
}

func (m *model) scrollToCurrentResult() {
	nextResult := m.searchResults[m.currentResultIndex]
	m.scrollViewportToLine(nextResult.Line)
}

func (m *model) navigateToNextResult() {
	if !m.hasSearchResults() {
		return
	}
	m.incrementSearchIndex()
	m.scrollToCurrentResult()
	m.highlightMatches()
	m.setSize(m.width, m.height)
}

func (m *model) navigateToPreviousResult() {
	if !m.hasSearchResults() {
		return
	}
	m.decrementSearchIndex()
	m.scrollToCurrentResult()
	m.highlightMatches()
	m.setSize(m.width, m.height)
}

func (m *model) handleNavigationForward(msg tea.Msg) tea.Cmd {
	if m.navigationMode {
		m.navigateToNextResult()
		return m.updateViewPort(msg)
	}
	return m.updateTextArea(msg)
}

func (m *model) handleNavigationBackwards(msg tea.Msg) tea.Cmd {
	if m.navigationMode {
		m.navigateToPreviousResult()
		return m.updateViewPort(msg)
	}
	return m.updateTextArea(msg)
}

func (m *model) handleNavigationActivation() tea.Cmd {
	if m.isSearching {
		m.textarea.Blur()
		m.navigationMode = true
		return nil
	}
	return nil
}

func (m *model) handleSearchActivation(msg tea.Msg) tea.Cmd {
	if m.isSearching {
		return m.updateTextArea(msg)
	}
	if !m.isSearching {
		m.setShowSearch(true)
		return nil
	}
	return nil
}

func (m *model) updateViewPort(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return cmd
}

func (m *model) searchCounter() string {
	counter := fmt.Sprintf("%d/%d", m.currentResultIndex+1, len(m.searchResults))
	if !m.hasSearchResults() {
		return " 0!"
	}
	return counter
}

func (m *model) setSize(width, height int) {
	m.viewport.Width = width

	contentHeight := height - statusBarHeight
	if m.showHelp {
		contentHeight -= m.helpHeight()
	}

	if m.isSearching {
		counter := m.searchCounter()
		contentHeight -= searchBoxHeight
		counterWidth := lipgloss.Width(counter)
		textareaWidth := width - counterWidth - 1
		m.textarea.SetWidth(textareaWidth)
		m.textarea.SetHeight(searchBoxHeight)
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
