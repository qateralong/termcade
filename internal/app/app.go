// Package app is the per-session Bubble Tea program: the intro, player setup,
// the lobby, and hosting whichever game the player launches.
package app

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"termcade/internal/games"
	"termcade/internal/hub"
	"termcade/internal/store"
	"termcade/internal/ui/theme"
)

// Minimum terminal size the lobby is designed for.
const (
	MinWidth  = 80
	MinHeight = 24
)

// Deps are the shared, server-wide services every session uses.
type Deps struct {
	Store *store.Store
	Hub   *hub.Hub
	Games *games.Registry
	Log   *log.Logger
}

// Identity describes the SSH connection behind a session.
type Identity struct {
	SessionID   string
	User        string // SSH username, used to suggest a nickname
	Fingerprint string // SHA256 key fingerprint; empty for guests
}

// Guest reports whether the session has no public key to remember it by.
func (id Identity) Guest() bool { return id.Fingerprint == "" }

type screen int

const (
	screenIntro screen = iota
	screenSetup
	screenLobby
	screenGame
)

// App is the root model of a session. It is used through a pointer so the
// server can call Close when the session ends.
type App struct {
	deps Deps
	th   *theme.Theme
	id   Identity

	send func(tea.Msg) // delivers messages into the running program

	player *store.Player // nil for guests and players not yet registered
	name   string
	color  string

	screen        screen
	width, height int
	frame         int // animation frame counter
	returning     bool

	intro intro
	setup setupForm
	lobby lobby

	game   tea.Model
	gameID string

	leave func() // leaves the hub; nil until joined

	toast      string
	toastErr   bool
	toastUntil time.Time
}

// New creates the app for one session. It looks up the player by key
// fingerprint; send must deliver messages into the session's program and may
// be called from other goroutines.
func New(deps Deps, th *theme.Theme, id Identity, send func(tea.Msg)) *App {
	m := &App{
		deps:   deps,
		th:     th,
		id:     id,
		send:   send,
		color:  theme.DefaultPlayerColor,
		width:  MinWidth,
		height: MinHeight,
	}
	if !id.Guest() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		p, err := deps.Store.PlayerByFingerprint(ctx, id.Fingerprint)
		switch {
		case err == nil:
			if p2, err := deps.Store.RecordLogin(ctx, p.ID); err == nil {
				p = p2
			}
			m.player = p
			m.name = p.Name
			m.color = p.Color
			if !theme.ValidPlayerColor(m.color) {
				m.color = theme.DefaultPlayerColor
			}
			m.returning = true
		case !errors.Is(err, store.ErrNotFound):
			deps.Log.Error("load player", "err", err)
		}
	}
	m.lobby = newLobby()
	return m
}

// Close releases everything the session holds. Safe to call more than once.
func (m *App) Close() {
	m.leaveGame()
	if m.leave != nil {
		m.leave()
	}
}

// leaveGame tells the current game, if any, that the player is gone.
func (m *App) leaveGame() {
	if l, ok := m.game.(games.Leaver); ok {
		l.Leave()
	}
	m.game, m.gameID = nil, ""
}

// Name returns the player's current display name, or "" before setup.
func (m *App) Name() string { return m.name }

// Color returns the player's color key.
func (m *App) Color() string { return m.color }

// Init implements tea.Model.
func (m *App) Init() tea.Cmd {
	return tea.Batch(tea.SetWindowTitle(theme.Name), m.intro.init())
}

type toastExpiredMsg struct{}

// notify shows a short message in the status bar.
func (m *App) notify(text string, isErr bool) tea.Cmd {
	const ttl = 3 * time.Second
	m.toast, m.toastErr, m.toastUntil = text, isErr, time.Now().Add(ttl)
	return tea.Tick(ttl, func(time.Time) tea.Msg { return toastExpiredMsg{} })
}

// Update implements tea.Model.
func (m *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.screen == screenGame {
			return m, m.updateGame(msg)
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case toastExpiredMsg:
		if !time.Now().Before(m.toastUntil) {
			m.toast = ""
		}
		return m, nil

	// Hub updates keep the lobby current no matter which screen is active.
	case hub.PresenceMsg:
		m.lobby.members = msg.Members
		return m, nil
	case hub.ChatMsg:
		m.lobby.addMessage(msg.Message)
		return m, nil

	case games.ExitMsg:
		return m, m.exitGame()
	}

	switch m.screen {
	case screenIntro:
		return m, m.updateIntro(msg)
	case screenSetup:
		return m, m.updateSetup(msg)
	case screenLobby:
		return m, m.updateLobby(msg)
	case screenGame:
		return m, m.updateGame(msg)
	}
	return m, nil
}

// View implements tea.Model.
func (m *App) View() string {
	if m.screen == screenGame && m.game != nil {
		return m.game.View()
	}
	if m.width < MinWidth || m.height < MinHeight {
		return m.viewTooSmall()
	}
	switch m.screen {
	case screenIntro:
		return m.viewIntro()
	case screenSetup:
		return m.viewSetup()
	default:
		return m.viewLobby()
	}
}

// finishIntro moves on from the intro to setup or straight into the lobby.
func (m *App) finishIntro() tea.Cmd {
	if m.player != nil {
		cmd := m.enterLobby()
		return tea.Batch(cmd, m.notify("Welcome back, "+m.name+"!", false))
	}
	return m.openSetup(false)
}

// enterLobby joins the hub (first time only) and shows the lobby.
func (m *App) enterLobby() tea.Cmd {
	m.screen = screenLobby
	if m.leave == nil {
		history, leave := m.deps.Hub.Join(hub.Member{
			SessionID: m.id.SessionID,
			Name:      m.name,
			Color:     m.color,
			Guest:     m.player == nil,
		}, func(msg any) { m.send(msg) })
		m.leave = leave
		for _, msg := range history {
			m.lobby.addMessage(msg)
		}
		m.lobby.members = m.deps.Hub.Members()
	}
	m.lobby.tickGen++
	return tea.Batch(m.lobbyTick(), m.lobby.focusGames())
}

// launch starts a game for this player.
func (m *App) launch(g games.Game) tea.Cmd {
	info := g.Info()
	if !g.Available() {
		return m.notify(info.Name+" is still in the workshop. Check back soon!", false)
	}
	model := g.Join(games.Session{
		Player: games.Player{
			SessionID: m.id.SessionID,
			ID:        m.playerID(),
			Name:      m.name,
			Color:     m.color,
			Guest:     m.player == nil,
		},
		Theme:  m.th,
		Width:  m.width,
		Height: m.height,
		Send:   m.send,
	})
	if model == nil {
		return m.notify("Couldn't start "+info.Name+", sorry.", true)
	}
	m.game, m.gameID, m.screen = model, info.ID, screenGame
	m.deps.Hub.SetLocation(m.id.SessionID, info.ID)
	return tea.Batch(model.Init(), func() tea.Msg {
		return tea.WindowSizeMsg{Width: m.width, Height: m.height}
	})
}

func (m *App) updateGame(msg tea.Msg) tea.Cmd {
	if m.game == nil {
		return m.exitGame()
	}
	var cmd tea.Cmd
	m.game, cmd = m.game.Update(msg)
	return cmd
}

func (m *App) exitGame() tea.Cmd {
	if m.screen != screenGame {
		return nil
	}
	m.leaveGame()
	m.deps.Hub.SetLocation(m.id.SessionID, hub.LocationLobby)
	return m.enterLobby()
}

func (m *App) playerID() int64 {
	if m.player == nil {
		return 0
	}
	return m.player.ID
}

// gameName resolves a hub location to a display name.
func (m *App) gameName(location string) string {
	if location == hub.LocationLobby {
		return "lobby"
	}
	if g, ok := m.deps.Games.Get(location); ok {
		return strings.ToLower(g.Info().Name)
	}
	return location
}

func (m *App) viewTooSmall() string {
	t := m.th
	size := t.Error.Bold(true).Render(itoa(m.width) + "×" + itoa(m.height))
	need := t.Success.Bold(true).Render(itoa(MinWidth) + "×" + itoa(MinHeight))
	body := lipgloss.JoinVertical(lipgloss.Center,
		t.Gradient(theme.SmallLogo(), theme.LogoGradient, 12, 0, true),
		"",
		t.Bold.Render("Your terminal is a little small"),
		t.Dim.Render("current ")+size+t.Dim.Render("  ·  needed ")+need,
		"",
		t.Faded.Render("resize the window or zoom out"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}
