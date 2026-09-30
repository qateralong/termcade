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
		Games: games.NewRegistry(games.Catalog()...),
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

func TestSetupFlowCreatesPlayer(t *testing.T) {
	m := newTestApp(t, Identity{SessionID: "s1", User: "neo", Fingerprint: "SHA256:abc"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// Any key skips the intro; a new player lands on the setup form with
	// their SSH username suggested as the nickname.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenSetup {
		t.Fatalf("screen = %v, want setup", m.screen)
	}
	if got := m.setup.name.Value(); got != "neo" {
		t.Fatalf("suggested name = %q", got)
	}

	// Pick the next color and submit.
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenLobby {
		t.Fatalf("screen = %v, want lobby (err %q)", m.screen, m.setup.err)
	}
	if m.player == nil || m.player.Name != "neo" || m.player.Color != theme.PlayerColors[1].Key {
		t.Fatalf("player not saved: %+v", m.player)
	}
	if members := m.deps.Hub.Members(); len(members) != 1 || members[0].Name != "neo" {
		t.Fatalf("hub members = %+v", members)
	}

	// A second session with the same key is recognized and skips setup.
	again := New(m.deps, m.th, Identity{SessionID: "s2", Fingerprint: "SHA256:abc"}, func(tea.Msg) {})
	defer again.Close()
	again.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if again.screen != screenLobby || again.name != "neo" || again.player.Logins != 2 {
		t.Fatalf("returning player: screen=%v name=%q player=%+v", again.screen, again.name, again.player)
	}
}

func TestComingSoonGameDoesNotLaunch(t *testing.T) {
	m := newTestApp(t, Identity{SessionID: "s1"})
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
