package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/hub"
	"termcade/internal/ui/theme"
)

var itoa = strconv.Itoa

type focus int

const (
	focusGames focus = iota
	focusChat
)

const (
	lobbyFrame   = 120 * time.Millisecond
	chatBacklog  = 100
	chatInputCap = hub.MaxMessageLen
)

type lobby struct {
	focus    focus
	selected int
	offset   int // first visible game row
	help     bool

	chat     []hub.ChatMessage
	input    textinput.Model
	inputSet bool // input styles are applied lazily, once the theme is known

	members []hub.Member
	tickGen int
}

// lobbyTickMsg drives the lobby animations. gen identifies the tick chain so
// re-entering the lobby never leaves two chains running.
type lobbyTickMsg struct{ gen int }

func newLobby() lobby { return lobby{} }

func (m *App) lobbyTick() tea.Cmd {
	gen := m.lobby.tickGen
	return tea.Tick(lobbyFrame, func(time.Time) tea.Msg { return lobbyTickMsg{gen: gen} })
}

func (l *lobby) addMessage(msg hub.ChatMessage) {
	if n := len(l.chat); n > 0 && l.chat[n-1].ID >= msg.ID {
		return // already have it (history and live updates can overlap)
	}
	l.chat = append(l.chat, msg)
	if len(l.chat) > chatBacklog {
		l.chat = append(l.chat[:0:0], l.chat[len(l.chat)-chatBacklog:]...)
	}
}

func (l *lobby) focusGames() tea.Cmd {
	l.focus = focusGames
	l.input.Blur()
	return nil
}

func (m *App) ensureInput() {
	l := &m.lobby
	if l.inputSet {
		return
	}
	t := m.th
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = chatInputCap
	in.Placeholder = "press tab to chat"
	in.TextStyle = t.Base
	in.PlaceholderStyle = t.Faded
	in.Cursor.Style = t.Fg(theme.Pink)
	in.Cursor.TextStyle = t.Base
	l.input = in
	l.inputSet = true
}

func (m *App) updateLobby(msg tea.Msg) tea.Cmd {
	m.ensureInput()
	l := &m.lobby

	switch msg := msg.(type) {
	case lobbyTickMsg:
		if msg.gen != l.tickGen {
			return nil // stale chain
		}
		m.frame++
		return m.lobbyTick()

	case tea.KeyMsg:
		if l.help {
			switch msg.String() {
			case "?", "esc", "q", "enter":
				l.help = false
			}
			return nil
		}
		if l.focus == focusChat {
			return m.updateChatKeys(msg)
		}
		return m.updateGamesKeys(msg)
	}

	if l.focus == focusChat {
		var cmd tea.Cmd
		l.input, cmd = l.input.Update(msg)
		return cmd
	}
	return nil
}

func (m *App) updateGamesKeys(msg tea.KeyMsg) tea.Cmd {
	l := &m.lobby
	all := m.deps.Games.All()
	switch msg.String() {
	case "up", "k":
		if l.selected > 0 {
			l.selected--
		}
	case "down", "j":
		if l.selected < len(all)-1 {
			l.selected++
		}
	case "home", "g":
		l.selected = 0
	case "end", "G":
		l.selected = len(all) - 1
	case "enter", " ":
		if l.selected < len(all) {
			return m.launch(all[l.selected])
		}
	case "tab", "c", "t", "/":
		l.focus = focusChat
		l.input.Placeholder = "say something nice"
		return l.input.Focus()
	case "p":
		return m.openSetup(true)
	case "?":
		l.help = true
	case "q":
		return tea.Quit
	}
	return nil
}

func (m *App) updateChatKeys(msg tea.KeyMsg) tea.Cmd {
	l := &m.lobby
	switch msg.String() {
	case "esc", "tab", "shift+tab":
		l.input.Placeholder = "press tab to chat"
		return l.focusGames()
	case "enter":
		text := l.input.Value()
		if strings.TrimSpace(text) == "" {
			return nil
		}
		if err := m.deps.Hub.Say(m.id.SessionID, text); err != nil {
			return m.notify(err.Error(), true)
		}
		l.input.Reset()
		return nil
	}
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	return cmd
}

// playersIn counts members at a location.
func (m *App) playersIn(location string) int {
	n := 0
	for _, mem := range m.lobby.members {
		if mem.Location == location {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// View

func (m *App) viewLobby() string {
	m.ensureInput()
	if m.lobby.help {
		return m.viewHelp()
	}
	w, h := m.width, m.height

	header := m.viewHeader(w)
	footer := m.viewFooter(w)
	avail := h - 2

	bottomH := clamp(avail*2/5, 8, 14)
	topH := avail - bottomH
	leftW := clamp(w*3/10, 28, 38)
	rightW := w - leftW

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.viewGameList(leftW, topH),
		m.viewGameDetail(rightW, topH),
	)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top,
		m.viewOnline(leftW, bottomH),
		m.viewChat(rightW, bottomH),
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, top, bottom, footer)
}

func (m *App) viewHeader(w int) string {
	t := m.th
	phase := float64(m.frame) * 0.015
	left := " " + t.Gradient(theme.SmallLogo(), theme.LogoGradient, 24, phase, true)
	if w >= 100 {
		left += t.Faded.Render("  ·  " + theme.Tagline)
	}

	online := len(m.lobby.members)
	right := t.Success.Render("●") + t.Dim.Render(" "+itoa(online)+" online") +
		t.Faded.Render("   ") +
		t.Fg(theme.PlayerHex(m.color)).Render("◆ ") + t.PlayerName(m.name, m.color)
	if m.player == nil {
		right += t.Faded.Render(" guest")
	}
	return spread(left, right+" ", w)
}

func (m *App) viewFooter(w int) string {
	t := m.th
	if m.toast != "" {
		style := t.Success
		icon := "✓ "
		if m.toastErr {
			style, icon = t.Error, "✗ "
		}
		return " " + fit(style.Render(icon+m.toast), w-1)
	}
	var h string
	if m.lobby.focus == focusChat {
		h = m.hints("enter", "send", "tab", "back to games", "esc", "leave chat")
	} else {
		h = m.hints("↑↓", "choose", "enter", "play", "tab", "chat", "p", "profile", "?", "help", "q", "quit")
	}
	return " " + fit(h, w-1)
}

func (m *App) viewGameList(w, h int) string {
	t := m.th
	l := &m.lobby
	all := m.deps.Games.All()
	innerW := w - 4
	rows := h - 2

	if l.selected >= len(all) {
		l.selected = len(all) - 1
	}
	if l.selected < l.offset {
		l.offset = l.selected
	}
	if l.selected >= l.offset+rows {
		l.offset = l.selected - rows + 1
	}

	lines := make([]string, 0, rows)
	for i := l.offset; i < len(all) && len(lines) < rows; i++ {
		g := all[i]
		info := g.Info()
		sel := i == l.selected
		bg := func(s lipgloss.Style) lipgloss.Style {
			if sel {
				return s.Background(t.Highlight)
			}
			return s
		}

		marker := bg(t.Base).Render("  ")
		if sel {
			marker = bg(t.Fg(theme.Pink).Bold(true)).Render("▌ ")
		}
		icon := bg(t.Fg(info.Accent[0])).Render(info.Icon + " ")
		name := bg(t.Base).Render(info.Name)
		if sel {
			name = bg(t.Bold).Render(info.Name)
		}

		var status string
		switch n := m.playersIn(info.ID); {
		case !g.Available():
			status = bg(t.Faded).Render("soon ")
		case n > 0:
			status = bg(t.Success).Render("● " + itoa(n) + " ")
		default:
			status = bg(t.Dim).Render("play ")
		}

		left := marker + icon + name
		gap := innerW - lipgloss.Width(left) - lipgloss.Width(status)
		if gap < 1 {
			gap = 1
		}
		lines = append(lines, left+bg(t.Base).Render(strings.Repeat(" ", gap))+status)
	}

	title := "GAMES"
	if len(all) > rows {
		title += " " + itoa(l.selected+1) + "/" + itoa(len(all))
	}
	return m.panel(title, strings.Join(lines, "\n"), w, h, l.focus == focusGames)
}

func (m *App) viewGameDetail(w, h int) string {
	t := m.th
	all := m.deps.Games.All()
	if len(all) == 0 {
		return m.panel("", t.Dim.Render("No games installed."), w, h, false)
	}
	g := all[m.lobby.selected]
	info := g.Info()
	innerW := w - 4
	innerH := h - 2
	accent := info.Accent
	if len(accent) == 0 {
		accent = theme.LogoGradient
	}

	// Title row: NAME ............ REALTIME · 2–8 PLAYERS
	name := t.Gradient(strings.ToUpper(info.Name), accent, 16, float64(m.frame)*0.01, true)
	meta := t.Dim.Render(strings.ToUpper(string(info.Kind))) + t.Faded.Render(" · ") +
		t.Dim.Render(info.Players+" players")
	head := []string{spread(name, meta, innerW), t.Dim.Italic(true).Render(info.Tagline)}

	var art []string
	if len(info.Art) > 0 {
		maxW := 0
		for _, line := range info.Art {
			maxW = max(maxW, lipgloss.Width(line))
		}
		art = strings.Split(t.Gradient(strings.Join(info.Art, "\n"), accent, max(maxW, 1), 0, false), "\n")
	}

	desc := wrap(info.Description, innerW)
	for i := range desc {
		desc[i] = t.Base.Render(desc[i])
	}

	var ctrl []string
	for i := 0; i+1 < len(info.Controls); i += 2 {
		ctrl = append(ctrl, t.Key.Render(info.Controls[i])+" "+t.Dim.Render(info.Controls[i+1]))
	}
	controls := strings.Join(ctrl, t.Faded.Render("   "))

	var action string
	if g.Available() {
		btn := t.R.NewStyle().Background(lipgloss.Color(theme.Pink)).
			Foreground(lipgloss.Color("#1A1325")).Bold(true).Padding(0, 2).Render("▶ PLAY")
		action = btn + t.Faded.Render("  press ") + t.Key.Render("enter")
		if n := m.playersIn(info.ID); n > 0 {
			action += t.Faded.Render("  ·  ") + t.Success.Render(itoa(n)+" playing now")
		}
	} else {
		action = t.Fg(theme.Amber).Bold(true).Render("◌ COMING SOON") +
			t.Faded.Render("  this cabinet is still being built")
	}

	// Assemble top-down, dropping the art and trimming the description if
	// the panel is too short. The action row is pinned to the bottom.
	build := func(withArt bool, descLines int) []string {
		out := append([]string{}, head...)
		if withArt && len(art) > 0 {
			out = append(out, "")
			out = append(out, art...)
		}
		out = append(out, "")
		out = append(out, desc[:descLines]...)
		if controls != "" {
			out = append(out, "", controls)
		}
		return out
	}
	// Each candidate must leave one row for the action.
	fits := func(b []string) bool { return len(b)+1 <= innerH }
	body := build(true, len(desc))
	if !fits(body) {
		body = build(false, len(desc))
	}
	for n := len(desc) - 1; !fits(body) && n >= 0; n-- {
		body = build(false, n)
	}
	for len(body) < innerH-1 {
		body = append(body, "")
	}
	body = append(body[:min(len(body), innerH-1)], action)

	return m.panel("DETAILS", strings.Join(body, "\n"), w, h, false)
}

func (m *App) viewOnline(w, h int) string {
	t := m.th
	innerW := w - 4
	rows := h - 2
	members := m.lobby.members

	lines := make([]string, 0, rows)
	for i, mem := range members {
		if len(lines) == rows-1 && len(members)-i > 1 {
			lines = append(lines, t.Faded.Render("+"+itoa(len(members)-i)+" more"))
			break
		}
		name := t.Fg(theme.PlayerHex(mem.Color)).Render("● ") + t.PlayerName(mem.Name, mem.Color)
		if mem.SessionID == m.id.SessionID {
			name += t.Faded.Render(" you")
		} else if mem.Guest {
			name += t.Faded.Render(" guest")
		}
		where := t.Faded.Render(m.gameName(mem.Location))
		if mem.Location != hub.LocationLobby {
			where = t.Dim.Render("▸ " + m.gameName(mem.Location))
		}
		lines = append(lines, spread(name, where, innerW))
	}
	return m.panel("ONLINE · "+itoa(len(members)), strings.Join(lines, "\n"), w, h, false)
}

func (m *App) viewChat(w, h int) string {
	t := m.th
	l := &m.lobby
	innerW := w - 4
	msgRows := h - 2 - 2 // leave room for the divider and the input line
	now := time.Now()

	// Render messages bottom-up until the area is full.
	var lines []string
	for i := len(l.chat) - 1; i >= 0 && len(lines) < msgRows; i-- {
		msg := l.chat[i]
		stamp := t.Faded.Render(padLeft(ago(msg.Time, now), 3) + " ")
		const stampW = 4
		var block []string
		if msg.System {
			for j, part := range wrap(msg.Text, innerW-stampW-2) {
				prefix := strings.Repeat(" ", stampW)
				if j == 0 {
					prefix = stamp
				}
				block = append(block, prefix+t.Faded.Italic(true).Render("· "+part))
			}
		} else {
			nameW := lipgloss.Width(msg.From) + 1
			for j, part := range wrap(msg.Text, innerW-stampW-nameW) {
				prefix := strings.Repeat(" ", stampW+nameW)
				if j == 0 {
					prefix = stamp + t.PlayerName(msg.From, msg.Color) + " "
				}
				block = append(block, prefix+t.Base.Render(part))
			}
		}
		lines = append(block, lines...)
	}
	if len(l.chat) == 0 {
		lines = []string{t.Faded.Render("It's quiet in here. Say hi!")}
	}
	if len(lines) > msgRows {
		lines = lines[len(lines)-msgRows:]
	}
	for len(lines) < msgRows {
		lines = append([]string{""}, lines...)
	}

	focused := l.focus == focusChat
	promptStyle := t.Faded
	if focused {
		promptStyle = t.Fg(theme.Pink).Bold(true)
	}
	l.input.Width = innerW - 3
	lines = append(lines,
		t.R.NewStyle().Foreground(t.Border).Render(strings.Repeat("─", innerW)),
		promptStyle.Render("› ")+l.input.View(),
	)
	return m.panel("LOBBY CHAT", strings.Join(lines, "\n"), w, h, focused)
}

func (m *App) viewHelp() string {
	t := m.th
	row := func(k, desc string) string {
		return t.Key.Render(padRight(k, 12)) + t.Base.Render(desc)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		t.Gradient(theme.SmallLogo(), theme.LogoGradient, 12, float64(m.frame)*0.015, true),
		"",
		t.Title.Render("KEYS"),
		row("↑ ↓  j k", "choose a game"),
		row("enter", "play the selected game"),
		row("tab", "switch between games and chat"),
		row("p", "edit your name and color"),
		row("?", "show or hide this help"),
		row("q", "leave the arcade"),
		"",
		t.Title.Render("ABOUT"),
		t.Base.Render("Termcade is a tiny multiplayer arcade that lives"),
		t.Base.Render("in your terminal. Your profile is tied to your SSH"),
		t.Base.Render("key, so there are no passwords and no sign-up."),
		"",
		t.Faded.Render("press ? or esc to close"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, t.Modal.Render(body))
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func padLeft(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

func padRight(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
