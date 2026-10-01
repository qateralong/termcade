package multi

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termcade/internal/games"
	"termcade/internal/ui/layout"
	"termcade/internal/ui/theme"
)

// frameMsg asks the session to redraw because the room changed.
type frameMsg struct{}

// model is one player's view of a room.
type model struct {
	g *Game
	s games.Session

	room *Room
	seat int
	w, h int

	wake chan struct{}
	stop chan struct{}

	recorded bool // this room's result has been saved
}

func newModel(g *Game, s games.Session) *model {
	m := &model{g: g, s: s, w: s.Width, h: s.Height}
	m.join()
	return m
}

// join sits the player down in a room and starts relaying its updates.
func (m *model) join() {
	m.wake = make(chan struct{}, 1)
	m.stop = make(chan struct{})
	wake, stop, send := m.wake, m.stop, m.s.Send
	notify := func() {
		select {
		case wake <- struct{}{}:
		default: // a redraw is already pending
		}
	}
	m.room, m.seat = m.g.seatFor(m.s.Player, notify)
	m.recorded = false
	if send == nil {
		return
	}
	go func() {
		for {
			select {
			case <-wake:
				send(frameMsg{})
			case <-stop:
				return
			}
		}
	}()
}

// Leave implements games.Leaver.
func (m *model) Leave() {
	if m.room == nil {
		return
	}
	m.room.leave(m.seat)
	close(m.stop)
	m.room = nil
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		return m, m.key(msg.String())
	case frameMsg:
		m.recordResult()
	}
	return m, nil
}

// recordResult saves this player's finishing place once the match is over.
func (m *model) recordResult() {
	if m.recorded || m.room == nil || m.s.Results == nil {
		return
	}
	r := m.room
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.phase != Finished {
		return
	}
	m.recorded = true
	for i, st := range r.results {
		if st.Seat == m.seat {
			m.s.Results.Record(games.Result{
				Game: r.g.cfg.Info.ID, Multiplayer: true,
				Place: i + 1, Seats: len(r.seats), Won: i == 0,
				Duration: time.Since(r.startedAt),
			})
			return
		}
	}
}

func (m *model) phase() Phase {
	m.room.mu.RLock()
	defer m.room.mu.RUnlock()
	return m.room.phase
}

func (m *model) key(k string) tea.Cmd {
	if m.room == nil {
		return games.Exit
	}
	switch m.phase() {
	case Finished:
		switch k {
		case "enter", " ", "r":
			m.Leave()
			m.join()
		case "q", "esc":
			return games.Exit
		}
	case Playing:
		if k == "q" || k == "esc" {
			return games.Exit
		}
		m.room.mu.Lock()
		if m.room.phase == Playing {
			m.room.world.Input(m.seat, k)
		}
		m.room.mu.Unlock()
	default:
		if k == "q" || k == "esc" {
			return games.Exit
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// View

func (m *model) View() string {
	if m.room == nil {
		return ""
	}
	t := m.s.Theme
	r := m.room
	r.mu.RLock()
	defer r.mu.RUnlock()

	var body string
	switch r.phase {
	case Waiting:
		body = m.viewWaiting()
	case Countdown:
		left := r.g.cfg.Countdown - time.Since(r.startedAt)
		n := int(left/time.Second) + 1
		body = layout.Overlay(r.world.View(m.seat, t), t.Modal.Padding(0, 3).Render(bigDigit(t, n)))
	case Playing:
		body = r.world.View(m.seat, t)
	case Finished:
		body = layout.Overlay(r.world.View(m.seat, t), m.viewResults())
	}

	bw, bh := lipgloss.Width(body), lipgloss.Height(body)
	if m.w < bw || m.h < bh+2 {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center,
			t.Bold.Render(r.g.cfg.Info.Name+" needs a bigger terminal")+"\n"+
				t.Dim.Render("need "+strconv.Itoa(bw)+"×"+strconv.Itoa(bh+2)+", have "+strconv.Itoa(m.w)+"×"+strconv.Itoa(m.h)))
	}
	screen, _, _ := layout.Frame(m.viewHeader(), body, m.viewFooter(), m.w, m.h)
	return screen
}

func (m *model) viewHeader() string {
	t := m.s.Theme
	r := m.room
	info := r.g.cfg.Info
	accent := info.Accent
	if len(accent) == 0 {
		accent = theme.LogoGradient
	}
	left := " " + t.Fg(accent[0]).Render(info.Icon) + " " +
		t.Gradient(strings.ToUpper(info.Name), accent, 12, 0, true) +
		t.Faded.Render("  ·  room "+strconv.Itoa(r.id))

	var parts []string
	if r.world != nil {
		for _, s := range r.world.Stats(m.seat) {
			parts = append(parts, t.Faded.Render(s.Label+" ")+t.Bold.Render(s.Value))
		}
	}
	return layout.Spread(left, strings.Join(parts, t.Faded.Render("   "))+" ", m.w)
}

func (m *model) viewFooter() string {
	t := m.s.Theme
	var line string
	switch m.room.phase {
	case Playing, Countdown:
		line = layout.KeyLine(t, append(append([]string{}, m.g.cfg.Info.Controls...), "q", "leave")...)
	case Finished:
		line = layout.KeyLine(t, "enter", "play again", "q", "back to lobby")
	default:
		line = layout.KeyLine(t, "q", "back to lobby")
	}
	return " " + ansi.Truncate(line, m.w-1, "…")
}

func (m *model) viewWaiting() string {
	t := m.s.Theme
	r := m.room
	info := r.g.cfg.Info
	accent := info.Accent
	if len(accent) == 0 {
		accent = theme.LogoGradient
	}

	seats := []string{t.Title.Render("PLAYERS")}
	for i, s := range r.seats {
		num := t.Faded.Render(strconv.Itoa(i+1) + ". ")
		switch {
		case s == nil:
			seats = append(seats, num+t.Faded.Render("waiting…"))
		default:
			line := num + t.Fg(s.color).Bold(true).Render("● "+s.name)
			if i == m.seat {
				line += t.Faded.Render(" you")
			}
			seats = append(seats, line)
		}
	}

	ctl := []string{t.Title.Render("CONTROLS")}
	for i := 0; i+1 < len(info.Controls); i += 2 {
		ctl = append(ctl, t.Key.Render(layout.PadRight(info.Controls[i], 8))+t.Base.Render(info.Controls[i+1]))
	}
	ctl = append(ctl, t.Key.Render(layout.PadRight("q", 8))+t.Base.Render("leave"))

	left := time.Until(r.deadline).Round(time.Second)
	if left < 0 {
		left = 0
	}
	status := t.Fg(theme.Amber).Bold(true).Render("starting in "+strconv.Itoa(int(left.Seconds()))+"s") +
		t.Faded.Render("  ·  bots take the empty seats")

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, seats...),
		"      ",
		lipgloss.JoinVertical(lipgloss.Left, ctl...),
	)
	return t.Modal.Padding(1, 4).Render(lipgloss.JoinVertical(lipgloss.Center,
		t.Gradient(layout.Spaced(strings.ToUpper(info.Name)), accent, 16, 0, true),
		t.Dim.Italic(true).Render(info.Tagline),
		"",
		cols,
		"",
		status,
	))
}

var ordinals = []string{"1st", "2nd", "3rd", "4th", "5th", "6th", "7th", "8th"}

func (m *model) viewResults() string {
	t := m.s.Theme
	r := m.room
	lines := []string{}
	place := -1
	for i, st := range r.results {
		if st.Seat == m.seat {
			place = i
		}
	}
	switch place {
	case 0:
		lines = append(lines, t.Fg(theme.Amber).Bold(true).Render("★ YOU WIN! ★"))
	case -1:
		lines = append(lines, t.Title.Render("RESULTS"))
	default:
		lines = append(lines, t.Bold.Render("You finished "+ordinals[min(place, len(ordinals)-1)]))
	}
	lines = append(lines, "")
	medals := []string{theme.Amber, "#C9D1E0", "#E0A060"}
	for i, st := range r.results {
		s := r.seats[st.Seat]
		pos := ordinals[min(i, len(ordinals)-1)]
		posStyle := t.Faded
		if i < len(medals) {
			posStyle = t.Fg(medals[i]).Bold(true)
		}
		name := s.name
		if !s.human || s.left {
			name += " (bot)"
		}
		row := posStyle.Render(layout.PadRight(pos, 5)) +
			t.Fg(s.color).Bold(true).Render(layout.PadRight(name, 18)) +
			t.Dim.Render(layout.PadLeft(st.Detail, 12))
		if st.Seat == m.seat {
			row += t.Faded.Render(" ◀")
		} else {
			row += "  "
		}
		lines = append(lines, row)
	}
	lines = append(lines, "", layout.KeyLine(t, "enter", "play again", "q", "leave"))
	return t.Modal.Padding(1, 3).Render(lipgloss.JoinVertical(lipgloss.Center, lines...))
}

var digits = map[int][]string{
	3: {"███", "  █", "███", "  █", "███"},
	2: {"███", "  █", "███", "█  ", "███"},
	1: {" █ ", "██ ", " █ ", " █ ", "███"},
}

func bigDigit(t *theme.Theme, n int) string {
	d, ok := digits[n]
	if !ok {
		return t.Title.Render("GO!")
	}
	var rows []string
	for _, row := range d {
		var b strings.Builder
		for _, c := range row {
			if c == '█' {
				b.WriteString("██")
			} else {
				b.WriteString("  ")
			}
		}
		rows = append(rows, b.String())
	}
	return lipgloss.JoinVertical(lipgloss.Center,
		t.Dim.Bold(true).Render("GET READY"),
		"",
		t.Gradient(strings.Join(rows, "\n"), theme.LogoGradient, 6, 0, true),
	)
}
