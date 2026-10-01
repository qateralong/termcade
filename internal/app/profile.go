package app

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/store"
	"termcade/internal/ui/layout"
	"termcade/internal/ui/theme"
)

// profileState is what the profile screen shows; it's loaded on open.
type profileState struct {
	stats  []store.GameStats
	bests  map[string]store.Bests
	keys   []store.Key
	keySel int
	tab    int // 0 overview, 1 computers
}

func (m *App) openProfile() tea.Cmd {
	m.screen = screenProfile
	m.profile.tab = 0
	m.loadProfile()
	return nil
}

func (m *App) loadProfile() {
	if m.player == nil {
		m.profile = profileState{}
		return
	}
	ctx, cancel := dbctx()
	defer cancel()
	var err error
	ps := &m.profile
	if ps.stats, err = m.deps.Store.GameStats(ctx, m.player.ID); err != nil {
		m.deps.Log.Error("load stats", "err", err)
	}
	if ps.bests, err = m.deps.Store.BestScores(ctx, m.player.ID); err != nil {
		m.deps.Log.Error("load bests", "err", err)
	}
	if ps.keys, err = m.deps.Store.Keys(ctx, m.player.ID); err != nil {
		m.deps.Log.Error("load keys", "err", err)
	}
	ps.keySel = min(ps.keySel, max(0, len(ps.keys)-1))
}

func (m *App) keyLinked() bool {
	for _, k := range m.profile.keys {
		if k.Fingerprint == m.id.Fingerprint {
			return true
		}
	}
	return false
}

func (m *App) updateProfile(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	k := key.String()
	switch k {
	case "esc", "q", "p":
		if m.profile.tab == 1 && k == "esc" {
			m.profile.tab = 0
			return nil
		}
		return m.enterLobby()
	case "e":
		return m.openForm(formEdit)
	}

	if m.player == nil { // guest
		switch k {
		case "r":
			return m.openForm(formRegister)
		case "l":
			if m.leave != nil {
				m.leave()
				m.leave = nil
			}
			m.name = ""
			return m.openForm(formLogin)
		}
		return nil
	}

	ps := &m.profile
	switch k {
	case "w":
		return m.openForm(formPassword)
	case "c", "tab":
		ps.tab = 1 - ps.tab
	case "o":
		detail := "You can log back in with your name and password."
		if !m.player.HasPassword {
			detail = "You have no password yet, so you won't be able to log back in from another computer. Set one first with w."
		}
		if m.id.Fingerprint != "" {
			detail += " This computer will forget you."
		}
		return m.ask("Log out of "+m.name+"?", detail, m.logout, nil)
	}

	if ps.tab == 1 {
		switch k {
		case "up", "k":
			ps.keySel = max(0, ps.keySel-1)
		case "down", "j":
			ps.keySel = min(len(ps.keys)-1, ps.keySel+1)
		case "a":
			if m.id.Fingerprint != "" && !m.keyLinked() {
				ctx, cancel := dbctx()
				defer cancel()
				if err := m.deps.Store.LinkKey(ctx, m.player.ID, m.id.Fingerprint); err != nil {
					m.deps.Log.Error("link key", "err", err)
					return m.notify("Couldn't remember this computer.", true)
				}
				m.loadProfile()
				return m.notify("This computer will sign you in automatically.", false)
			}
		case "x", "delete", "backspace":
			if ps.keySel < len(ps.keys) {
				fp := ps.keys[ps.keySel].Fingerprint
				forget := func() tea.Cmd {
					ctx, cancel := dbctx()
					defer cancel()
					if err := m.deps.Store.UnlinkKey(ctx, m.player.ID, fp); err != nil {
						m.deps.Log.Error("unlink key", "err", err)
					}
					m.screen = screenProfile
					m.loadProfile()
					return m.notify("Computer forgotten.", false)
				}
				what := "That computer will have to log in with your password next time."
				if fp == m.id.Fingerprint {
					what = "This is the computer you're using now. Next time you'll have to log in with your password."
				}
				if !m.player.HasPassword {
					what += " You have no password yet, so set one first (w) or you may lose access."
				}
				return m.ask("Forget "+shortFingerprint(fp)+"?", what, forget, nil)
			}
		}
	}
	return nil
}

func (m *App) viewProfile() string {
	t := m.th
	if m.player == nil {
		return m.viewGuestProfile()
	}
	p := m.player
	ps := &m.profile

	online := time.Duration(p.SecondsOnline) * time.Second
	if !m.onlineSince.IsZero() {
		online += time.Since(m.onlineSince)
	}
	pw := t.Success.Render("✓ password set")
	if !p.HasPassword {
		pw = t.Fg(theme.Amber).Render("✗ no password: press w to set one")
	}
	head := lipgloss.JoinVertical(lipgloss.Left,
		t.Fg(theme.PlayerHex(m.color)).Render("◆ ")+t.PlayerName(p.Name, m.color),
		t.Faded.Render("member since "+p.CreatedAt.Format("2 Jan 2006")+"  ·  "+
			itoa(p.Logins)+" visits  ·  "+humanDuration(online)+" in the arcade"),
		pw+t.Faded.Render("  ·  ")+t.Dim.Render(plural(len(ps.keys), "computer")+" remembered"),
	)

	var body string
	var hints string
	if ps.tab == 1 {
		body = m.viewKeys()
		hints = m.hints("↑↓", "select", "x", "forget", "a", "remember this one", "tab", "stats", "esc", "back")
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.viewGameStats(), "    ", m.viewAchievements())
		hints = m.hints("e", "edit", "w", "password", "c", "computers", "o", "log out", "esc", "back")
	}

	card := t.Modal.Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		t.Title.Render("PROFILE"), "", head, "", body))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, card, hints))
}

func (m *App) viewGuestProfile() string {
	t := m.th
	card := t.Modal.Width(56).Render(lipgloss.JoinVertical(lipgloss.Left,
		t.Title.Render("YOU'RE PLAYING AS A GUEST"),
		"",
		t.Fg(theme.PlayerHex(m.color)).Render("◆ ")+t.PlayerName(m.name, m.color),
		"",
		t.Base.Width(48).Render("Guests don't keep stats, achievements or high scores, and lose their name when they leave. Create an account to keep everything and play from any computer."),
		"",
		t.Key.Render("r")+t.Faded.Render("  create an account (keeps your name)"),
		t.Key.Render("l")+t.Faded.Render("  log in to an existing account"),
		t.Key.Render("e")+t.Faded.Render("  change your guest name or color"),
	))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, card, m.hints("esc", "back")))
}

// viewGameStats is a table of games played, most played first.
func (m *App) viewGameStats() string {
	t := m.th
	ps := &m.profile
	lines := []string{
		t.Dim.Bold(true).Render(layout.PadRight("GAME", 14) + layout.PadLeft("PLAYED", 7) + layout.PadLeft("WINS", 6) + layout.PadLeft("BEST", 9)),
	}
	if len(ps.stats) == 0 {
		lines = append(lines, t.Faded.Render("nothing yet, go play something!"))
	}
	maxRows := max(3, m.height-16)
	for i, s := range ps.stats {
		if i >= maxRows {
			lines = append(lines, t.Faded.Render("+"+itoa(len(ps.stats)-i)+" more"))
			break
		}
		name, wins, best := s.Game, t.Faded.Render(layout.PadLeft("–", 6)), t.Faded.Render(layout.PadLeft("–", 9))
		if g, ok := m.deps.Games.Get(s.Game); ok {
			info := g.Info()
			name = info.Name
			if info.Mode == games.Multiplayer {
				wins = t.Success.Render(layout.PadLeft(itoa(s.Wins), 6))
			}
		}
		if b := m.bestFor(s.Game); b != "" {
			best = t.Bold.Render(layout.PadLeft(b, 9))
		}
		lines = append(lines, t.Base.Render(layout.PadRight(trimName(name, 13), 14))+
			t.Dim.Render(layout.PadLeft(itoa(s.Plays), 7))+wins+best)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// bestFor formats the best score across a game's boards (variants).
func (m *App) bestFor(game string) string {
	if b, ok := m.profile.bests[game]; ok {
		return solo.Thousands(b.Max)
	}
	// Variant boards, e.g. "minesweeper:hard": show the fastest time.
	bestTime := -1
	for board, b := range m.profile.bests {
		if strings.HasPrefix(board, game+":") && (bestTime < 0 || b.Min < bestTime) {
			bestTime = b.Min
		}
	}
	if bestTime >= 0 {
		return solo.Duration(bestTime)
	}
	return ""
}

func (m *App) viewAchievements() string {
	t := m.th
	sum := m.summary()
	got := 0
	var rows []string
	for _, a := range achievements {
		if a.done(sum) {
			got++
			rows = append(rows, t.Fg(theme.Amber).Render("★ ")+t.Bold.Render(a.name))
		} else if m.width >= 112 {
			rows = append(rows, t.Faded.Render("☆ "+a.name+"  "+a.desc))
		} else {
			rows = append(rows, t.Faded.Render("☆ "+a.name))
		}
	}
	maxRows := max(3, m.height-16)
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	title := t.Dim.Bold(true).Render("ACHIEVEMENTS ") + t.Fg(theme.Amber).Render(itoa(got)+"/"+itoa(len(achievements)))
	return lipgloss.JoinVertical(lipgloss.Left, append([]string{title}, rows...)...)
}

func (m *App) viewKeys() string {
	t := m.th
	ps := &m.profile
	lines := []string{t.Dim.Bold(true).Render("REMEMBERED COMPUTERS"),
		t.Faded.Render("These sign you in automatically. Anywhere else, use your password."), ""}
	if len(ps.keys) == 0 {
		lines = append(lines, t.Faded.Render("none"))
	}
	for i, k := range ps.keys {
		line := shortFingerprint(k.Fingerprint) + t.Faded.Render("  added "+k.AddedAt.Format("2 Jan 2006"))
		if k.Fingerprint == m.id.Fingerprint {
			line += t.Success.Render("  ← this computer")
		}
		if i == ps.keySel {
			line = t.Fg(theme.Pink).Bold(true).Render("▸ ") + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	if m.id.Fingerprint != "" && !m.keyLinked() {
		lines = append(lines, "", t.Faded.Render("This computer isn't remembered. Press ")+t.Key.Render("a")+t.Faded.Render(" to add it."))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func trimName(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}

// humanDuration formats a duration as "3h 20m", "12m" or "<1m".
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "<1m"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	}
	return strconv.Itoa(int(d.Hours())) + "h " + strconv.Itoa(int(d.Minutes())%60) + "m"
}
