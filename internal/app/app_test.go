package app

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"

	"termcade/internal/games"
	"termcade/internal/games/lineup"
	"termcade/internal/hub"
	"termcade/internal/store"
	"termcade/internal/ui/theme"
)

func newTestApp(t *testing.T, id Identity) *App {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	deps := Deps{
		Store: st,
		Hub:   hub.New(),
		Games: games.NewRegistry(lineup.All()...),
		Log:   log.New(io.Discard),
	}
	m := New(deps, theme.New(r), id, func(tea.Msg) {})
	t.Cleanup(m.Close)
	return m
}

// assertFrame checks that a view fills exactly w×h cells, so nothing wraps
// or scrolls in a real terminal.
func assertFrame(t *testing.T, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Fatalf("%dx%d: view has %d lines", w, h, len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != w {
			t.Fatalf("%dx%d: line %d is %d cells wide: %q", w, h, i, got, line)
		}
	}
}

func TestLobbyLayoutFitsEverySize(t *testing.T) {
	m := newTestApp(t, Identity{SessionID: "s1", User: "tester"})
	m.name = "tester"
	m.enterLobby()
	m.deps.Hub.Say("s1", strings.Repeat("a long chat message ", 20))

	for _, f := range []focus{focusGames, focusChat} {
		m.lobby.focus = f
		for w := MinWidth; w <= 220; w += 7 {
			for h := MinHeight; h <= 70; h += 5 {
				m.Update(tea.WindowSizeMsg{Width: w, Height: h})
				for sel := range m.deps.Games.All() {
					m.lobby.selected = sel
					assertFrame(t, m.View(), w, h)
				}
			}
		}
	}
}

func TestComingSoonGameDoesNotLaunch(t *testing.T) {
	m := newTestApp(t, Identity{SessionID: "s1"})
	m.deps.Games = games.NewRegistry(games.ComingSoon(games.Info{ID: "soon", Name: "Soon", Icon: "*"}))
	m.name = "guesty"
	m.enterLobby()
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenLobby || !strings.Contains(m.toast, "workshop") {
		t.Fatalf("screen=%v toast=%q", m.screen, m.toast)
	}
}

// fakeGame is a playable game that records what happens to its players.
type fakeGame struct{ left *int }

func (fakeGame) Info() games.Info {
	return games.Info{ID: "fake", Name: "Fake", Icon: "*", Accent: []string{theme.Lime}}
}
func (fakeGame) Available() bool                { return true }
func (g fakeGame) Join(games.Session) tea.Model { return &fakeModel{left: g.left} }

type fakeModel struct{ left *int }

func (*fakeModel) Init() tea.Cmd { return nil }
func (f *fakeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "q" {
		return f, games.Exit
	}
	return f, nil
}
func (*fakeModel) View() string { return "in the fake game" }
func (f *fakeModel) Leave()     { *f.left++ }

func TestGameLifecycle(t *testing.T) {
	left := 0
	m := newTestApp(t, Identity{SessionID: "s1"})
	m.deps.Games = games.NewRegistry(fakeGame{left: &left})
	m.name = "player"
	m.enterLobby()

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenGame || m.View() != "in the fake game" {
		t.Fatalf("game did not start: screen=%v", m.screen)
	}
	if loc := m.deps.Hub.Members()[0].Location; loc != "fake" {
		t.Fatalf("location = %q", loc)
	}

	// q inside the game returns games.Exit, which brings us back.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m.Update(cmd())
	if m.screen != screenLobby || left != 1 {
		t.Fatalf("after exit: screen=%v left=%d", m.screen, left)
	}
	if loc := m.deps.Hub.Members()[0].Location; loc != "lobby" {
		t.Fatalf("location after exit = %q", loc)
	}

	// Disconnecting mid-game also notifies the game, exactly once.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Close()
	m.Close()
	if left != 2 {
		t.Fatalf("left = %d after disconnect", left)
	}
}

func TestShutdownNotice(t *testing.T) {
	m := newTestApp(t, Identity{SessionID: "s1"})
	m.name = "player"
	m.enterLobby()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := m.Update(hub.ShutdownMsg{Reason: "The arcade is restarting"})
	if !m.ShuttingDown() || !strings.Contains(m.View(), "The arcade is restarting") {
		t.Fatal("shutdown notice not shown")
	}
	if cmd == nil {
		t.Fatal("expected a delayed quit command")
	}
	// Input is ignored while the notice is up.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if m.screen != screenLobby {
		t.Fatalf("screen changed to %v during shutdown", m.screen)
	}
}
