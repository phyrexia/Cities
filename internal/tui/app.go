package tui

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#4A9EFF")).
			Padding(0, 2)

	cityNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF88"))

	statLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	statValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	happyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF88"))
	sadStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))

	proposalStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4A9EFF")).
			Padding(0, 1).
			Width(60)

	selectedProposalStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FFD700")).
				Background(lipgloss.Color("#1a1a2e")).
				Padding(0, 1).
				Width(60)

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)

	barFill  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF88"))
	barEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
)

// ─── Message Types ──────────────────────────────────────────────────────────

type wsConnectedMsg struct{ conn *websocket.Conn }
type wsErrorMsg struct{ err error }
type wsMessageMsg struct{ data []byte }
type tickMsg time.Time

// ─── Model ───────────────────────────────────────────────────────────────────

type screen int

const (
	screenCity screen = iota
	screenProposals
	screenTrade
	screenLog
)

// CityState holds the current city data from the server.
type CityState struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	MayorName  string  `json:"mayor_name"`
	Happiness  float64 `json:"happiness"`
	Treasury   int64   `json:"treasury"`
	TaxRate    float64 `json:"tax_rate"`
	Population struct {
		Total         int `json:"total"`
		Workers       int `json:"workers"`
		Families      int `json:"families"`
		Entrepreneurs int `json:"entrepreneurs"`
		Students      int `json:"students"`
		Homeless      int `json:"homeless"`
	} `json:"population"`
	Stats struct {
		UnemploymentRate int   `json:"unemployment_rate"`
		CrimeRate        int   `json:"crime_rate"`
		EducationLevel   int   `json:"education_level"`
		HealthLevel      int   `json:"health_level"`
		GDP              int64 `json:"gdp"`
	} `json:"stats"`
}

// Proposal is a simplified initiative for display.
type Proposal struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Cost        int64   `json:"cost"`
	Effects     Effects `json:"effects"`
}

type Effects struct {
	PopulationDelta int     `json:"population_delta"`
	HappinessDelta  float64 `json:"happiness_delta"`
	TreasuryDelta   int64   `json:"treasury_delta"`
	Description     string  `json:"description"`
}

// Model is the bubbletea model.
type Model struct {
	playerID    string
	cityID      string
	serverURL   string
	conn        *websocket.Conn
	city        *CityState
	proposals   []Proposal
	heartbeatID string
	selected    int
	screen      screen
	logs        []string
	decided     bool
	width       int
	height      int
	err         error
}

// New creates a new TUI model.
func New(playerID, cityID, serverURL string) *Model {
	return &Model{
		playerID:  playerID,
		cityID:    cityID,
		serverURL: serverURL,
		screen:    screenCity,
		logs:      []string{"Connecting to coordinator..."},
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.connectWS(),
		tickEvery(time.Second),
	)
}

func (m *Model) connectWS() tea.Cmd {
	return func() tea.Msg {
		wsURL := fmt.Sprintf("%s/ws?player_id=%s&city_id=%s",
			strings.Replace(m.serverURL, "http", "ws", 1), m.playerID, m.cityID)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return wsErrorMsg{err: fmt.Errorf("WS connect: %w", err)}
		}
		return wsConnectedMsg{conn: conn}
	}
}

func (m *Model) listenWS() tea.Cmd {
	return func() tea.Msg {
		_, data, err := m.conn.ReadMessage()
		if err != nil {
			return wsErrorMsg{err: err}
		}
		return wsMessageMsg{data: data}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Every(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		return m, tickEvery(time.Second)

	case wsConnectedMsg:
		m.conn = msg.conn
		m.addLog("Connected to coordinator")
		return m, m.listenWS()

	case wsErrorMsg:
		m.err = msg.err
		m.addLog(fmt.Sprintf("Error: %v", msg.err))
		return m, nil

	case wsMessageMsg:
		m.handleServerMessage(msg.data)
		return m, m.listenWS()
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "1":
		m.screen = screenCity
	case "2":
		m.screen = screenProposals
	case "3":
		m.screen = screenTrade
	case "4":
		m.screen = screenLog

	case "up", "k":
		if m.screen == screenProposals && m.selected > 0 {
			m.selected--
		}

	case "down", "j":
		if m.screen == screenProposals && m.selected < len(m.proposals)-1 {
			m.selected++
		}

	case "enter", " ":
		if m.screen == screenProposals && len(m.proposals) > 0 && !m.decided {
			m.submitDecision()
		}
	}
	return m, nil
}

func (m *Model) submitDecision() {
	if m.conn == nil || len(m.proposals) == 0 {
		return
	}
	chosen := m.proposals[m.selected]
	payload := map[string]interface{}{
		"type": "decision",
		"payload": map[string]string{
			"heartbeat_id":  m.heartbeatID,
			"initiative_id": chosen.ID,
		},
	}
	data, _ := json.Marshal(payload)
	if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		m.addLog(fmt.Sprintf("Failed to submit decision: %v", err))
		return
	}
	m.decided = true
	m.addLog(fmt.Sprintf("Decision submitted: %s", chosen.Title))
}

func (m *Model) handleServerMessage(data []byte) {
	var envelope struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		log.Printf("ws parse error: %v", err)
		return
	}

	switch envelope.Type {
	case "heartbeat":
		var hb struct {
			ID       string     `json:"id"`
			CityID   string     `json:"city_id"`
			Round    int        `json:"round"`
			Proposals []Proposal `json:"proposals"`
		}
		if err := json.Unmarshal(envelope.Payload, &hb); err != nil {
			return
		}
		m.heartbeatID = hb.ID
		m.proposals = hb.Proposals
		m.selected = 0
		m.decided = false
		m.screen = screenProposals
		m.addLog(fmt.Sprintf("Heartbeat! Round %d — %d proposals received", hb.Round, len(hb.Proposals)))

	case "city_update":
		var update struct {
			CityID string     `json:"city_id"`
			City   *CityState `json:"city"`
		}
		if err := json.Unmarshal(envelope.Payload, &update); err != nil {
			return
		}
		if update.City != nil {
			m.city = update.City
			m.addLog(fmt.Sprintf("City updated — pop: %d, treasury: %d", update.City.Population.Total, update.City.Treasury))
		}

	case "world_event":
		var event struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal(envelope.Payload, &event); err != nil {
			return
		}
		m.addLog(fmt.Sprintf("[WORLD] %s", event.Description))
	}
}

func (m *Model) addLog(msg string) {
	m.logs = append(m.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	if len(m.logs) > 50 {
		m.logs = m.logs[len(m.logs)-50:]
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.err != nil {
		return sadStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	}

	header := m.renderHeader()
	nav := m.renderNav()
	var content string
	switch m.screen {
	case screenCity:
		content = m.renderCityView()
	case screenProposals:
		content = m.renderProposals()
	case screenTrade:
		content = m.renderTrade()
	case screenLog:
		content = m.renderLog()
	}
	footer := statLabelStyle.Render("q: quit  |  1-4: switch view  |  ↑↓: navigate  |  Enter: select")

	return lipgloss.JoinVertical(lipgloss.Left, header, nav, content, footer)
}

func (m *Model) renderHeader() string {
	title := titleStyle.Render("CITIES — Social Simulator")
	if m.city != nil {
		cityInfo := cityNameStyle.Render(fmt.Sprintf("Mayor of %s", m.city.Name))
		return lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", cityInfo)
	}
	return title
}

func (m *Model) renderNav() string {
	tabs := []string{"[1] City", "[2] Proposals", "[3] Trade", "[4] Log"}
	var rendered []string
	for i, tab := range tabs {
		if screen(i) == m.screen {
			rendered = append(rendered, happyStyle.Bold(true).Render(tab))
		} else {
			rendered = append(rendered, statLabelStyle.Render(tab))
		}
	}
	return strings.Join(rendered, "  ") + "\n"
}

func (m *Model) renderCityView() string {
	if m.city == nil {
		return statLabelStyle.Render("Waiting for city data from coordinator...")
	}
	c := m.city
	var sb strings.Builder

	sb.WriteString(cityNameStyle.Render(fmt.Sprintf("═══ %s ═══", c.Name)) + "\n\n")

	// Stats grid
	sb.WriteString(renderStat("Treasury", fmt.Sprintf("%d ¢", c.Treasury)) + "\n")
	sb.WriteString(renderStat("Tax Rate", fmt.Sprintf("%.1f%%", c.TaxRate)) + "\n")
	sb.WriteString(renderStat("Happiness", renderBar(c.Happiness, 100)) + "\n")
	sb.WriteString("\n")

	sb.WriteString(statLabelStyle.Render("── Population ──") + "\n")
	sb.WriteString(renderStat("Total", fmt.Sprintf("%d", c.Population.Total)) + "\n")
	sb.WriteString(renderStat("Workers", fmt.Sprintf("%d", c.Population.Workers)) + "\n")
	sb.WriteString(renderStat("Families", fmt.Sprintf("%d", c.Population.Families)) + "\n")
	sb.WriteString(renderStat("Entrepreneurs", fmt.Sprintf("%d", c.Population.Entrepreneurs)) + "\n")
	sb.WriteString(renderStat("Students", fmt.Sprintf("%d", c.Population.Students)) + "\n")
	sb.WriteString(renderStat("Homeless", fmt.Sprintf("%d", c.Population.Homeless)) + "\n")
	sb.WriteString("\n")

	sb.WriteString(statLabelStyle.Render("── City Stats ──") + "\n")
	sb.WriteString(renderStat("Unemployment", renderBar(float64(c.Stats.UnemploymentRate), 100)) + "\n")
	sb.WriteString(renderStat("Crime Rate", renderBar(float64(c.Stats.CrimeRate), 100)) + "\n")
	sb.WriteString(renderStat("Education", renderBar(float64(c.Stats.EducationLevel), 100)) + "\n")
	sb.WriteString(renderStat("Health", renderBar(float64(c.Stats.HealthLevel), 100)) + "\n")
	sb.WriteString(renderStat("GDP", fmt.Sprintf("%d ¢", c.Stats.GDP)) + "\n")

	return sb.String()
}

func (m *Model) renderProposals() string {
	if len(m.proposals) == 0 {
		return statLabelStyle.Render("Waiting for the next heartbeat...\n\nThe coordinator will send proposals every 10 minutes.")
	}
	var sb strings.Builder
	if m.decided {
		sb.WriteString(happyStyle.Render("✓ Decision submitted! Waiting for next heartbeat.") + "\n\n")
	} else {
		sb.WriteString(warnStyle.Bold(true).Render("! HEARTBEAT — Choose an initiative (↑↓ + Enter):") + "\n\n")
	}

	for i, p := range m.proposals {
		style := proposalStyle
		prefix := "  "
		if i == m.selected {
			style = selectedProposalStyle
			prefix = "▶ "
		}

		icon := initiativeIcon(p.Type)
		content := fmt.Sprintf("%s%s %s\n%s\nCost: %d ¢  |  %s\nEffects: pop %+d  happy %+.1f  treasury %+d",
			prefix, icon, p.Title,
			wrapText(p.Description, 55),
			p.Cost,
			p.Type,
			p.Effects.PopulationDelta,
			p.Effects.HappinessDelta,
			p.Effects.TreasuryDelta,
		)
		sb.WriteString(style.Render(content) + "\n")
	}
	return sb.String()
}

func (m *Model) renderTrade() string {
	return statLabelStyle.Render("Trade interface coming soon.\n\nUse the REST API or web client for trading.")
}

func (m *Model) renderLog() string {
	var sb strings.Builder
	sb.WriteString(statLabelStyle.Render("Event Log:") + "\n\n")
	start := 0
	if len(m.logs) > 20 {
		start = len(m.logs) - 20
	}
	for _, entry := range m.logs[start:] {
		sb.WriteString(logStyle.Render(entry) + "\n")
	}
	return sb.String()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func renderStat(label, value string) string {
	return statLabelStyle.Render(fmt.Sprintf("%-18s", label)) + statValueStyle.Render(value)
}

func renderBar(value, max float64) string {
	pct := value / max
	filled := int(pct * 20)
	empty := 20 - filled
	bar := barFill.Render(strings.Repeat("█", filled)) + barEmpty.Render(strings.Repeat("░", empty))
	return bar + " " + fmt.Sprintf("%.0f%%", value)
}

func initiativeIcon(t string) string {
	icons := map[string]string{
		"HOUSING":    "🏠",
		"INDUSTRY":   "🏭",
		"EDUCATION":  "🎓",
		"HEALTH":     "🏥",
		"TAX_CHANGE": "💰",
		"TRADE_DEAL": "🤝",
		"INNOVATION": "💡",
		"POLICY":     "📋",
		"SECURITY":   "🚔",
		"GREEN":      "🌱",
	}
	if icon, ok := icons[t]; ok {
		return icon
	}
	return "📌"
}

func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}
	words := strings.Fields(text)
	var lines []string
	current := ""
	for _, word := range words {
		if len(current)+len(word)+1 > width {
			if current != "" {
				lines = append(lines, current)
			}
			current = word
		} else {
			if current == "" {
				current = word
			} else {
				current += " " + word
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}
