package solo

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termcade/internal/games"
	"termcade/internal/ui/theme"
)

type phase int

const (
	phaseTitle phase = iota
	phasePlaying
	phasePaused
	phaseOver
)

type tickMsg struct{ gen int }

type model struct {
	cfg Config
	s   games.Session
	rng *rand.Rand

	w, h    int
	phase   phase
	variant int

	eng  Engine
	gen  int // identifies the current tick chain
	last time.Time

	best    int
	hasBest bool
	top     []games.ScoreEntry
	record  bool

	// Where the play area was last drawn, for mapping mouse clicks.
	originX, originY int
}

func (m *model) th() *theme.Theme { return m.s.Theme }

func (m *model) board() string {
	id := m.cfg.Info.ID
	if len(m.cfg.Variants) > 0 {
		id += ":" + m.cfg.Variants[m.variant].Key
	}
	return id
}

func (m *model) variantKey() string {
	if len(m.cfg.Variants) == 0 {
		return ""
	}
	return m.cfg.Variants[m.variant].Key
}

// loadBoard refreshes the best score and leaderboard for the current board.
func (m *model) loadBoard() {
	if m.s.Scores == nil {
		return
	}
	m.best, m.hasBest = m.s.Scores.Best(m.board(), m.cfg.LowerIsBetter)
	m.top = m.s.Scores.Top(m.board(), m.cfg.LowerIsBetter, 5)
}

func (m *model) Init() tea.Cmd {
	if m.cfg.Mouse {
		return tea.EnableMouseCellMotion
	}
	return nil
}

func (m *model) tick() tea.Cmd {
	gen := m.gen
	return tea.Tick(m.cfg.Tick, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func (m *model) start() tea.Cmd {
	m.eng = m.cfg.New(m.s, m.variantKey(), m.rng)
	m.phase = phasePlaying
	m.record = false
	return m.resume()
}

func (m *model) resume() tea.Cmd {
	m.phase = phasePlaying
	m.gen++
	m.last = time.Now()
	return m.tick()
}

func (m *model) finish() {
	m.phase = phaseOver
	m.gen++ // stop ticking
	score := m.eng.Score()
	won := m.eng.State() == Won
	if m.s.Scores != nil && (won || (!m.cfg.OnlyWins && score > 0)) {
		m.best, m.record = m.s.Scores.Submit(m.board(), m.cfg.LowerIsBetter, score)
		m.hasBest = true
	}
	m.loadBoard()
}

func (m *model) exit() tea.Cmd {
	m.gen++
	if m.cfg.Mouse {
		return tea.Sequence(tea.DisableMouse, games.Exit)
	}
	return games.Exit
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		if msg.gen != m.gen || m.phase != phasePlaying {
			return m, nil
		}
		now := time.Now()
		dt := min(now.Sub(m.last), 250*time.Millisecond) // don't jump after a stall
		m.last = now
		m.eng.Update(dt)
		if m.eng.State() != Playing {
			m.finish()
			return m, nil
		}
		return m, m.tick()

	case tea.MouseMsg:
		if me, ok := m.eng.(MouseEngine); ok && m.phase == phasePlaying &&
			msg.Action == tea.MouseActionPress {
			me.Mouse(msg.X-m.originX, msg.Y-m.originY, msg.Button)
			if m.eng.State() != Playing {
				m.finish()
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m, m.key(msg.String())
	}
	return m, nil
}

func (m *model) key(k string) tea.Cmd {
	switch m.phase {
	case phaseTitle:
		switch k {
		case "enter", " ":
			return m.start()
		case "left", "h", "a":
			m.cycleVariant(-1)
		case "right", "l", "d", "tab":
			m.cycleVariant(1)
		case "q", "esc":
			return m.exit()
		}

	case phasePlaying:
		switch k {
		case "p", "esc", "q":
			m.phase = phasePaused
			m.gen++
			return nil
		}
		m.eng.Key(k)
		if m.eng.State() != Playing {
			m.finish()
		}

	case phasePaused:
		switch k {
		case "p", "esc", "enter", " ":
			return m.resume()
		case "r":
			return m.start()
		case "q":
			return m.exit()
		}

	case phaseOver:
		switch k {
		case "enter", " ", "r":
			return m.start()
		case "t":
			m.phase = phaseTitle
			m.loadBoard()
		case "q", "esc":
			return m.exit()
		}
	}
	return nil
}

func (m *model) cycleVariant(d int) {
	n := len(m.cfg.Variants)
	if n == 0 {
		return
	}
	m.variant = (m.variant + d + n) % n
	m.loadBoard()
}

// ---------------------------------------------------------------------------
// View

func (m *model) View() string {
	t := m.th()

	var body string
	switch m.phase {
	case phaseTitle:
		body = m.viewTitle()
	case phasePaused:
		body = overlay(m.eng.View(), m.card(
			t.Title.Render("PAUSED"),
			"",
			m.keyLine("p", "resume", "r", "restart", "q", "leave"),
		))
	case phaseOver:
		body = overlay(m.eng.View(), m.viewOver())
	default:
		body = m.eng.View()
	}

	bodyW, bodyH := lipgloss.Width(body), lipgloss.Height(body)
	if m.w < bodyW+2 || m.h < bodyH+2 {
		return m.viewTooSmall(bodyW+2, bodyH+2)
	}

	header := m.viewHeader()
	footer := m.viewFooter()

	// Center the body in the space between header and footer.
	space := m.h - 2
	top := (space - bodyH) / 2
	left := (m.w - bodyW) / 2
	m.originX, m.originY = left, 1+top

	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	lines := strings.Split(body, "\n")
	pad := strings.Repeat(" ", left)
	for i := 0; i < space; i++ {
		if j := i - top; j >= 0 && j < len(lines) {
			b.WriteString(pad + lines[j])
		}
		b.WriteByte('\n')
	}
	b.WriteString(footer)
	return b.String()
}

func (m *model) viewHeader() string {
	t := m.th()
	info := m.cfg.Info
	accent := info.Accent
	if len(accent) == 0 {
		accent = theme.LogoGradient
	}
	left := " " + t.Fg(accent[0]).Render(info.Icon) + " " +
		t.Gradient(strings.ToUpper(info.Name), accent, 12, 0, true)
	if len(m.cfg.Variants) > 0 {
		left += t.Faded.Render("  ·  " + m.cfg.Variants[m.variant].Name)
	}

	var parts []string
	stat := func(label, value string) {
		parts = append(parts, t.Faded.Render(label+" ")+t.Bold.Render(value))
	}
	if m.eng != nil && m.phase != phaseTitle {
		stat(m.cfg.ScoreLabel, m.cfg.FormatScore(m.eng.Score()))
		for _, s := range m.eng.Stats() {
			stat(s.Label, s.Value)
		}
	}
	if m.hasBest {
		stat("BEST", m.cfg.FormatScore(m.best))
	}
	right := strings.Join(parts, t.Faded.Render("   ")) + " "

	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left+" "+right, m.w, "")
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *model) viewFooter() string {
	var line string
	switch m.phase {
	case phaseTitle:
		pairs := []string{"enter", "start"}
		if len(m.cfg.Variants) > 1 {
			pairs = append(pairs, "←→", "difficulty")
		}
		line = m.keyLine(append(pairs, "q", "back to lobby")...)
	case phasePlaying:
		line = m.keyLine(append(append([]string{}, m.cfg.Info.Controls...), "p", "pause")...)
	case phasePaused:
		line = m.keyLine("p", "resume", "r", "restart", "q", "back to lobby")
	case phaseOver:
		line = m.keyLine("enter", "play again", "t", "title", "q", "back to lobby")
	}
	return " " + ansi.Truncate(line, m.w-1, "…")
}

func (m *model) keyLine(pairs ...string) string {
	t := m.th()
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, t.Key.Render(pairs[i])+" "+t.Faded.Render(pairs[i+1]))
	}
	return strings.Join(parts, t.Faded.Render("  ·  "))
}

func (m *model) card(lines ...string) string {
	return m.th().Modal.Padding(1, 4).Render(lipgloss.JoinVertical(lipgloss.Center, lines...))
}

func (m *model) viewTitle() string {
	t := m.th()
	info := m.cfg.Info
	accent := info.Accent
	if len(accent) == 0 {
		accent = theme.LogoGradient
	}

	name := t.Gradient(spaced(strings.ToUpper(info.Name)), accent, 16, 0, true)

	// Controls column.
	ctl := []string{t.Title.Render("CONTROLS")}
	keys := append(append([]string{}, info.Controls...), "p", "pause")
	for i := 0; i+1 < len(keys); i += 2 {
		ctl = append(ctl, t.Key.Render(padRight(keys[i], 8))+t.Base.Render(keys[i+1]))
	}

	// Leaderboard column.
	lb := []string{t.Title.Render("LEADERBOARD")}
	if len(m.top) == 0 {
		lb = append(lb, t.Faded.Render("no scores yet, be the first!"))
	}
	for i, e := range m.top {
		lb = append(lb, t.Faded.Render(strconv.Itoa(i+1)+". ")+
			t.PlayerName(padRight(e.Name, 16), e.Color)+t.Bold.Render(padLeft(m.cfg.FormatScore(e.Score), 9)))
	}
	if m.hasBest {
		lb = append(lb, "", t.Dim.Render(padRight("your best", 19))+t.Bold.Render(padLeft(m.cfg.FormatScore(m.best), 9)))
	}

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, ctl...),
		"      ",
		lipgloss.JoinVertical(lipgloss.Left, lb...),
	)

	parts := []string{name, t.Dim.Italic(true).Render(info.Tagline), "", cols}
	if n := len(m.cfg.Variants); n > 0 {
		v := m.cfg.Variants[m.variant]
		sel := t.Faded.Render("◀  ") + t.Fg(accent[0]).Bold(true).Render(strings.ToUpper(v.Name)) + t.Faded.Render("  ▶")
		parts = append(parts, "", sel, t.Faded.Render(v.Desc))
	}
	parts = append(parts, "", t.R.NewStyle().Background(lipgloss.Color(accent[0])).
		Foreground(lipgloss.Color("#1A1325")).Bold(true).Padding(0, 2).Render("▶ PRESS ENTER"))
	return m.card(parts...)
}

func (m *model) viewOver() string {
	t := m.th()
	title := t.Error.Bold(true).Render("GAME OVER")
	if m.eng.State() == Won {
		title = t.Success.Bold(true).Render("YOU WIN!")
	}
	lines := []string{
		title,
		"",
		t.Faded.Render(m.cfg.ScoreLabel+"  ") + t.Bold.Render(m.cfg.FormatScore(m.eng.Score())),
	}
	if m.record {
		lines = append(lines, t.Fg(theme.Amber).Bold(true).Render("★ NEW PERSONAL BEST ★"))
	} else if m.hasBest {
		lines = append(lines, t.Faded.Render("BEST  ")+t.Dim.Render(m.cfg.FormatScore(m.best)))
	}
	if len(m.top) > 0 {
		lines = append(lines, "", t.Title.Render("TOP "+strconv.Itoa(len(m.top))))
		for i, e := range m.top {
			lines = append(lines, t.Faded.Render(strconv.Itoa(i+1)+". ")+
				t.PlayerName(padRight(e.Name, 16), e.Color)+t.Bold.Render(padLeft(m.cfg.FormatScore(e.Score), 9)))
		}
	}
	lines = append(lines, "", m.keyLine("enter", "again", "q", "leave"))
	return m.card(lines...)
}

func (m *model) viewTooSmall(needW, needH int) string {
	t := m.th()
	body := lipgloss.JoinVertical(lipgloss.Center,
		t.Bold.Render(m.cfg.Info.Name+" needs a bigger terminal"),
		t.Dim.Render("current ")+t.Error.Render(strconv.Itoa(m.w)+"×"+strconv.Itoa(m.h))+
			t.Dim.Render("  ·  needed ")+t.Success.Render(strconv.Itoa(needW)+"×"+strconv.Itoa(needH)),
		"",
		t.Faded.Render("resize the window, or press q to leave"),
	)
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, body)
}

// overlay draws fg centered on top of bg.
func overlay(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	bw := lipgloss.Width(bg)
	fw := lipgloss.Width(fg)
	if fw > bw || len(fgLines) > len(bgLines) {
		return fg // the card doesn't fit; show it alone
	}
	x := (bw - fw) / 2
	y := (len(bgLines) - len(fgLines)) / 2
	for i, fl := range fgLines {
		line := bgLines[y+i]
		if w := ansi.StringWidth(line); w < bw {
			line += strings.Repeat(" ", bw-w)
		}
		left := ansi.Truncate(line, x, "")
		right := ansi.Cut(line, x+ansi.StringWidth(fl), bw)
		bgLines[y+i] = left + "\x1b[0m" + fl + "\x1b[0m" + right
	}
	return strings.Join(bgLines, "\n")
}

func spaced(s string) string {
	r := []rune(s)
	out := make([]string, len(r))
	for i, c := range r {
		out[i] = string(c)
	}
	return strings.Join(out, " ")
}

func padRight(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}
