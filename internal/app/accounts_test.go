package app

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"

	"termcade/internal/auth"
	"termcade/internal/games"
	"termcade/internal/games/lineup"
	"termcade/internal/hub"
	"termcade/internal/store"
	"termcade/internal/ui/theme"
)

// arcade is a shared server for several test sessions.
type arcade struct {
	deps Deps
	th   *theme.Theme
}

func newArcade(t *testing.T) *arcade {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return &arcade{
		deps: Deps{
			Store:  st,
			Hub:    hub.New(),
			Games:  games.NewRegistry(lineup.All()...),
			Log:    log.New(io.Discard),
			Logins: auth.NewLimiter(5, 60e9),
		},
		th: theme.New(r),
	}
}

// connect opens a session and skips the intro.
func (a *arcade) connect(t *testing.T, id Identity) *App {
	t.Helper()
	m := New(a.deps, a.th, id, func(tea.Msg) {})
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// fill types values into the current form's fields.
func fill(m *App, values ...string) {
	for i, v := range values {
		m.form.fields[i].input.SetValue(v)
	}
}

func TestRegisterThenLogInFromAnotherComputer(t *testing.T) {
	a := newArcade(t)

	// On the laptop: create an account.
	laptop := a.connect(t, Identity{SessionID: "s1", User: "neo", Fingerprint: "SHA256:laptop"})
	if laptop.screen != screenWelcome {
		t.Fatalf("new player should see the welcome screen, got %v", laptop.screen)
	}
	laptop.Update(key("1"))
	if laptop.screen != screenForm || laptop.formKind != formRegister {
		t.Fatal("1 should open the registration form")
	}
	if got := laptop.form.value(0); got != "neo" {
		t.Fatalf("suggested name = %q", got)
	}
	fill(laptop, "neo", "secret1", "secret2")
	laptop.submitRegister()
	if !strings.Contains(laptop.form.err, "match") {
		t.Fatalf("mismatched passwords accepted: %q", laptop.form.err)
	}
	fill(laptop, "neo", "secret1", "secret1")
	laptop.submitRegister()
	if laptop.screen != screenLobby || laptop.player == nil || !laptop.player.HasPassword {
		t.Fatalf("registration failed: screen=%v err=%q", laptop.screen, laptop.form.err)
	}
	laptop.Close()

	// On a desktop with a different key: log in, and remember it.
	desk := a.connect(t, Identity{SessionID: "s2", Fingerprint: "SHA256:desktop"})
	desk.Update(key("2"))
	fill(desk, "NEO", "wrong-password")
	desk.submitLogin()
	if desk.form.err != auth.ErrWrong.Error() || desk.player != nil {
		t.Fatalf("wrong password let us in: %q", desk.form.err)
	}
	fill(desk, "NEO", "secret1")
	desk.submitLogin()
	if desk.screen != screenConfirm {
		t.Fatal("should ask to remember this computer")
	}
	desk.Update(key("y"))
	if desk.screen != screenLobby || desk.name != "neo" {
		t.Fatalf("login failed: screen=%v name=%q", desk.screen, desk.name)
	}
	desk.Close()

	// Next time, the desktop signs straight in.
	again := a.connect(t, Identity{SessionID: "s3", Fingerprint: "SHA256:desktop"})
	if again.screen != screenLobby || again.name != "neo" {
		t.Fatalf("remembered computer didn't sign in: screen=%v", again.screen)
	}
}

func TestLoginLockout(t *testing.T) {
	a := newArcade(t)
	reg := a.connect(t, Identity{SessionID: "s1"})
	reg.Update(key("1"))
	fill(reg, "alice", "correct1", "correct1")
	reg.submitRegister()
	reg.Close()

	m := a.connect(t, Identity{SessionID: "s2"})
	m.Update(key("2"))
	for i := 0; i < 5; i++ {
		fill(m, "alice", "nope-nope")
		m.submitLogin()
	}
	fill(m, "alice", "correct1")
	m.submitLogin()
	if m.player != nil || m.form.err != auth.ErrLockedOut.Error() {
		t.Fatalf("lockout didn't stop the right password: err=%q", m.form.err)
	}
}

func TestGuestCanRegisterLater(t *testing.T) {
	a := newArcade(t)
	m := a.connect(t, Identity{SessionID: "s1", User: "visitor"})
	m.Update(key("3"))
	fill(m, "visitor")
	m.submitGuest()
	if m.screen != screenLobby || m.player != nil || m.name != "visitor" {
		t.Fatal("guest didn't reach the lobby")
	}
	if !a.deps.Hub.Members()[0].Guest {
		t.Fatal("hub should list a guest")
	}

	m.Update(key("p"))
	if m.screen != screenProfile {
		t.Fatal("p should open the profile")
	}
	m.Update(key("r"))
	if m.form.value(0) != "visitor" {
		t.Fatal("registration should keep the guest's name")
	}
	fill(m, "visitor", "password", "password")
	m.submitRegister()
	if m.player == nil || a.deps.Hub.Members()[0].Guest {
		t.Fatal("guest wasn't converted to a player")
	}
	if n := len(a.deps.Hub.Members()); n != 1 {
		t.Fatalf("converting should not rejoin the hub; %d members", n)
	}
}

func TestPasswordChangeAndLogout(t *testing.T) {
	a := newArcade(t)
	// A player from before passwords existed: key only.
	ctx, cancel := dbctx()
	defer cancel()
	a.deps.Store.CreatePlayer(ctx, "oldtimer", "lime", "", "SHA256:old")

	m := a.connect(t, Identity{SessionID: "s1", Fingerprint: "SHA256:old"})
	if m.player == nil || m.player.HasPassword {
		t.Fatal("key-only player should sign in without a password")
	}
	m.Update(key("p"))
	m.Update(key("w"))
	if m.form.title != "SET A PASSWORD" || len(m.form.fields) != 2 {
		t.Fatalf("form = %q with %d fields", m.form.title, len(m.form.fields))
	}
	fill(m, "brand-new", "brand-new")
	m.submitPassword()
	if !m.player.HasPassword || m.screen != screenProfile {
		t.Fatalf("password not set: err=%q", m.form.err)
	}

	// Changing it needs the current one.
	m.Update(key("w"))
	fill(m, "not-it", "another1", "another1")
	m.submitPassword()
	if m.form.err == "" {
		t.Fatal("changed password without the current one")
	}
	m.Update(key("esc"))

	// Log out: this computer forgets the account.
	m.Update(key("o"))
	m.Update(key("y"))
	if m.screen != screenWelcome || m.player != nil {
		t.Fatal("logout didn't return to the welcome screen")
	}
	if _, err := a.deps.Store.PlayerByFingerprint(ctx, "SHA256:old"); err == nil {
		t.Fatal("logout should unlink this computer")
	}
}

func TestProfileShowsResultsAndFits(t *testing.T) {
	a := newArcade(t)
	m := a.connect(t, Identity{SessionID: "s1", User: "stats"})
	m.Update(key("1"))
	fill(m, "stats", "password", "password")
	m.submitRegister()

	m.scores.Record(games.Result{Game: "tanks", Multiplayer: true, Place: 1, Seats: 4, Won: true})
	m.scores.Record(games.Result{Game: "blockfall", Seats: 1, Score: 12000})
	m.scores.Submit("blockfall", false, 12000)

	m.Update(key("p"))
	view := m.View()
	for _, want := range []string{"Tanks", "Blockfall", "12,000", "First Victory", "Block Master"} {
		if !strings.Contains(view, want) {
			t.Errorf("profile is missing %q", want)
		}
	}
	for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 50}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, tab := range []int{0, 1} {
			m.profile.tab = tab
			assertFrame(t, m.View(), size[0], size[1])
		}
	}
}

func TestFormsFitSmallTerminals(t *testing.T) {
	a := newArcade(t)
	m := a.connect(t, Identity{SessionID: "s1", Fingerprint: "SHA256:k"})
	for _, kind := range []formKind{formRegister, formLogin, formGuest} {
		m.openForm(kind)
		m.form.err = "something went wrong"
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			assertFrame(t, m.View(), size[0], size[1])
		}
	}
	m.openWelcome()
	assertFrame(t, m.View(), 120, 40)
}
