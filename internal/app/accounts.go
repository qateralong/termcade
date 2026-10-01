package app

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/auth"
	"termcade/internal/store"
	"termcade/internal/textutil"
	"termcade/internal/ui/theme"
)

// defaultLimiter guards logins when Deps doesn't provide one.
var defaultLimiter = auth.NewLimiter(5, time.Minute)

func (m *App) limiter() *auth.Limiter {
	if m.deps.Logins != nil {
		return m.deps.Logins
	}
	return defaultLimiter
}

func dbctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

var errSomething = errors.New("something went wrong, try again")

// ---------------------------------------------------------------------------
// Welcome: create an account, log in, or play as a guest.

var welcomeItems = []struct{ label, desc string }{
	{"Create an account", "pick a name and a password, play from any computer"},
	{"Log in", "already have an account? sign in with your password"},
	{"Play as a guest", "jump straight in; nothing is saved"},
}

func (m *App) openWelcome() tea.Cmd {
	m.screen = screenWelcome
	m.welcomeSel = 0
	return nil
}

func (m *App) updateWelcome(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "up", "k":
		m.welcomeSel = (m.welcomeSel + len(welcomeItems) - 1) % len(welcomeItems)
	case "down", "j", "tab":
		m.welcomeSel = (m.welcomeSel + 1) % len(welcomeItems)
	case "1":
		return m.openForm(formRegister)
	case "2":
		return m.openForm(formLogin)
	case "3":
		return m.openForm(formGuest)
	case "enter", " ":
		return m.openForm([]formKind{formRegister, formLogin, formGuest}[m.welcomeSel])
	case "q", "esc":
		return tea.Quit
	}
	return nil
}

func (m *App) viewWelcome() string {
	t := m.th
	var items []string
	for i, it := range welcomeItems {
		num := t.Faded.Render(itoa(i+1) + "  ")
		if i == m.welcomeSel {
			items = append(items,
				t.Fg(theme.Pink).Bold(true).Render("▸ ")+num+t.Bold.Render(it.label),
				"     "+t.Dim.Render(it.desc))
		} else {
			items = append(items, "  "+num+t.Base.Render(it.label), "     "+t.Faded.Render(it.desc))
		}
		if i < len(welcomeItems)-1 {
			items = append(items, "")
		}
	}
	box := t.Modal.Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{t.Title.Render("WELCOME, PLAYER"), ""}, items...)...))

	note := t.Faded.Render("◆ no SSH key: an account lets you keep your name and stats")
	if !m.id.Guest() {
		note = t.Faded.Render("◆ new computer? log in once and it will remember you")
	}

	logo := theme.BigLogo(theme.Name)
	if m.height < 32 || lipgloss.Width(logo) > m.width-4 {
		logo = theme.SmallLogo()
	}
	art := t.Gradient(logo, theme.LogoGradient, lipgloss.Width(logo), float64(m.frame)*0.01, true)
	body := lipgloss.JoinVertical(lipgloss.Center,
		art, t.Dim.Render(theme.Tagline), "", box, "",
		m.hints("↑↓", "choose", "enter", "select", "q", "quit"), note)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// ---------------------------------------------------------------------------
// Forms

type formKind int

const (
	formRegister formKind = iota
	formLogin
	formGuest
	formEdit
	formPassword
)

func (m *App) openForm(kind formKind) tea.Cmd {
	f := form{}
	suggested := m.name
	if suggested == "" {
		suggested = textutil.SuggestName(m.id.User)
		if suggested != "" && m.nameUnavailable(suggested) != nil {
			suggested = ""
		}
	}
	switch kind {
	case formRegister:
		f.title, f.submit = "CREATE AN ACCOUNT", "create account"
		f.intro = "You can log in with this from any computer."
		f.fields = []*field{
			m.nameField(suggested),
			m.passwordField("PASSWORD", "at least 6 characters"),
			m.passwordField("CONFIRM PASSWORD", "type it once more"),
			m.colorField(m.color),
		}
	case formLogin:
		f.title, f.submit = "LOG IN", "log in"
		f.intro = "Welcome back!"
		f.fields = []*field{m.nameField(""), m.passwordField("PASSWORD", "")}
		f.fields[0].help = ""
	case formGuest:
		f.title, f.submit = "PLAY AS A GUEST", "start playing"
		f.intro = "Your name is yours until you disconnect."
		f.fields = []*field{m.nameField(suggested), m.colorField(m.color)}
	case formEdit:
		f.title, f.submit = "EDIT PROFILE", "save"
		f.fields = []*field{m.nameField(m.name), m.colorField(m.color)}
	case formPassword:
		f.submit = "save password"
		if m.player != nil && m.player.HasPassword {
			f.title = "CHANGE PASSWORD"
			f.fields = []*field{
				m.passwordField("CURRENT PASSWORD", ""),
				m.passwordField("NEW PASSWORD", "at least 6 characters"),
				m.passwordField("CONFIRM NEW PASSWORD", "type it once more"),
			}
		} else {
			f.title = "SET A PASSWORD"
			f.intro = "Then you can log in as " + m.name + " from any computer."
			f.fields = []*field{
				m.passwordField("NEW PASSWORD", "at least 6 characters"),
				m.passwordField("CONFIRM PASSWORD", "type it once more"),
			}
		}
	}
	m.form, m.formKind = f, kind
	m.screen = screenForm
	return m.form.focusOn(0)
}

func (m *App) updateForm(msg tea.Msg) tea.Cmd {
	submit, cancel, cmd := m.form.update(msg)
	switch {
	case cancel:
		if m.signedIn() {
			return m.openProfile()
		}
		return m.openWelcome()
	case submit:
		switch m.formKind {
		case formRegister:
			return m.submitRegister()
		case formLogin:
			return m.submitLogin()
		case formGuest:
			return m.submitGuest()
		case formEdit:
			return m.submitEdit()
		case formPassword:
			return m.submitPassword()
		}
	}
	return cmd
}

func (m *App) viewFormScreen() string {
	var notes []string
	switch m.formKind {
	case formRegister:
		if !m.id.Guest() {
			notes = append(notes, m.th.Faded.Render("◆ this computer's SSH key will sign you in automatically"))
		}
	case formGuest:
		notes = append(notes, m.th.Fg(theme.Amber).Render("◆ guests don't keep stats or high scores"))
	}
	return m.viewForm(&m.form, notes...)
}

// signedIn reports whether the session has a name, as a player or guest.
func (m *App) signedIn() bool { return m.name != "" }

// nameUnavailable returns why name can't be used by this session, if at all.
func (m *App) nameUnavailable(name string) error {
	if err := textutil.ValidateName(name); err != nil {
		return err
	}
	if strings.EqualFold(name, m.name) {
		return nil // keeping your own name is always fine
	}
	ctx, cancel := dbctx()
	defer cancel()
	taken, err := m.deps.Store.NameTaken(ctx, name, m.playerID())
	if err != nil {
		m.deps.Log.Error("check name", "err", err)
		return errSomething
	}
	if taken || m.deps.Hub.NameOnline(name, m.id.SessionID) {
		return store.ErrNameTaken
	}
	return nil
}

func (m *App) submitRegister() tea.Cmd {
	f := &m.form
	name := strings.TrimSpace(f.value(0))
	if err := m.nameUnavailable(name); err != nil {
		return f.fail(0, err)
	}
	if err := auth.Validate(f.value(1), f.value(2)); err != nil {
		if errors.Is(err, auth.ErrMismatch) {
			return f.fail(2, err)
		}
		return f.fail(1, err)
	}
	hash, err := auth.Hash(f.value(1))
	if err != nil {
		m.deps.Log.Error("hash password", "err", err)
		return f.fail(-1, errSomething)
	}
	ctx, cancel := dbctx()
	defer cancel()
	p, err := m.deps.Store.CreatePlayer(ctx, name, f.colorKey(), hash, m.id.Fingerprint)
	if errors.Is(err, store.ErrNameTaken) {
		return f.fail(0, err)
	}
	if err != nil {
		m.deps.Log.Error("create player", "err", err)
		return f.fail(-1, errSomething)
	}
	return m.signIn(p, "Account created. Welcome to the arcade, "+p.Name+"!")
}

func (m *App) submitLogin() tea.Cmd {
	f := &m.form
	name := strings.TrimSpace(f.value(0))
	if name == "" {
		return f.fail(0, errors.New("enter your name"))
	}
	lim := m.limiter()
	if err := lim.Allowed(name); err != nil {
		return f.fail(-1, err)
	}
	ctx, cancel := dbctx()
	defer cancel()
	p, err := m.deps.Store.PlayerByName(ctx, name)
	var hash string
	if err == nil {
		hash, err = m.deps.Store.PasswordHash(ctx, p.ID)
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		m.deps.Log.Error("login lookup", "err", err)
		return f.fail(-1, errSomething)
	}
	if err != nil || !auth.Check(hash, f.value(1)) {
		lim.Fail(name)
		f.fields[1].input.SetValue("")
		return f.fail(1, auth.ErrWrong)
	}
	lim.Succeed(name)

	// Offer to remember this computer if its key isn't linked to them yet.
	if fp := m.id.Fingerprint; fp != "" {
		owner, err := m.deps.Store.PlayerByFingerprint(ctx, fp)
		if err != nil || owner.ID != p.ID {
			return m.ask("Remember this computer?",
				"Next time you connect from here, you'll be signed in as "+p.Name+" automatically.",
				func() tea.Cmd {
					ctx, cancel := dbctx()
					defer cancel()
					if err := m.deps.Store.LinkKey(ctx, p.ID, fp); err != nil {
						m.deps.Log.Error("link key", "err", err)
					}
					return m.signIn(p, "Welcome back, "+p.Name+"! This computer will remember you.")
				},
				func() tea.Cmd { return m.signIn(p, "Welcome back, "+p.Name+"!") })
		}
	}
	return m.signIn(p, "Welcome back, "+p.Name+"!")
}

func (m *App) submitGuest() tea.Cmd {
	f := &m.form
	name := strings.TrimSpace(f.value(0))
	if err := m.nameUnavailable(name); err != nil {
		return f.fail(0, err)
	}
	m.name, m.color = name, f.colorKey()
	cmd := m.enterLobby()
	return tea.Batch(cmd, m.notify("Welcome, "+name+"! Press p any time to create an account.", false))
}

func (m *App) submitEdit() tea.Cmd {
	f := &m.form
	name := strings.TrimSpace(f.value(0))
	color := f.colorKey()
	if err := m.nameUnavailable(name); err != nil {
		return f.fail(0, err)
	}
	if m.player != nil {
		ctx, cancel := dbctx()
		defer cancel()
		if err := m.deps.Store.UpdateProfile(ctx, m.player.ID, name, color); err != nil {
			if !errors.Is(err, store.ErrNameTaken) {
				m.deps.Log.Error("save profile", "err", err)
				err = errSomething
			}
			return f.fail(0, err)
		}
		m.player.Name, m.player.Color = name, color
	}
	changed := name != m.name || color != m.color
	m.name, m.color = name, color
	if changed && m.leave != nil {
		m.deps.Hub.Rename(m.id.SessionID, name, color)
	}
	cmd := m.openProfile()
	if changed {
		return tea.Batch(cmd, m.notify("Profile saved.", false))
	}
	return cmd
}

func (m *App) submitPassword() tea.Cmd {
	f := &m.form
	if m.player == nil {
		return m.openProfile()
	}
	ctx, cancel := dbctx()
	defer cancel()
	next, confirm, nextField := f.value(0), f.value(1), 0
	if m.player.HasPassword {
		lim := m.limiter()
		if err := lim.Allowed(m.name); err != nil {
			return f.fail(-1, err)
		}
		hash, err := m.deps.Store.PasswordHash(ctx, m.player.ID)
		if err != nil || !auth.Check(hash, f.value(0)) {
			lim.Fail(m.name)
			f.fields[0].input.SetValue("")
			return f.fail(0, errors.New("that's not your current password"))
		}
		lim.Succeed(m.name)
		next, confirm, nextField = f.value(1), f.value(2), 1
	}
	if err := auth.Validate(next, confirm); err != nil {
		if errors.Is(err, auth.ErrMismatch) {
			return f.fail(nextField+1, err)
		}
		return f.fail(nextField, err)
	}
	hash, err := auth.Hash(next)
	if err == nil {
		err = m.deps.Store.SetPasswordHash(ctx, m.player.ID, hash)
	}
	if err != nil {
		m.deps.Log.Error("set password", "err", err)
		return f.fail(-1, errSomething)
	}
	m.player.HasPassword = true
	return tea.Batch(m.openProfile(), m.notify("Password saved. Log in as "+m.name+" from any computer.", false))
}

// signIn makes p the session's player and heads to the lobby.
func (m *App) signIn(p *store.Player, greeting string) tea.Cmd {
	ctx, cancel := dbctx()
	defer cancel()
	if p2, err := m.deps.Store.RecordLogin(ctx, p.ID); err == nil {
		p = p2
	}
	m.player = p
	m.name = p.Name
	m.color = p.Color
	if !theme.ValidPlayerColor(m.color) {
		m.color = theme.DefaultPlayerColor
	}
	m.onlineSince = time.Now()
	if m.leave != nil {
		// A guest who just registered or logged in: update them in place.
		m.deps.Hub.Rename(m.id.SessionID, m.name, m.color)
		m.deps.Hub.SetGuest(m.id.SessionID, false)
	}
	return tea.Batch(m.enterLobby(), m.notify(greeting, false))
}

// saveOnlineTime adds the time since sign-in to the player's total.
func (m *App) saveOnlineTime() {
	if m.player == nil || m.onlineSince.IsZero() {
		return
	}
	ctx, cancel := dbctx()
	defer cancel()
	if err := m.deps.Store.AddOnlineTime(ctx, m.player.ID, time.Since(m.onlineSince)); err != nil {
		m.deps.Log.Error("save online time", "err", err)
	}
	m.player.SecondsOnline += int(time.Since(m.onlineSince) / time.Second)
	m.onlineSince = time.Now()
}

// logout signs the player out and makes this computer forget them.
func (m *App) logout() tea.Cmd {
	if m.player != nil {
		m.saveOnlineTime()
		if fp := m.id.Fingerprint; fp != "" {
			ctx, cancel := dbctx()
			defer cancel()
			if err := m.deps.Store.UnlinkKey(ctx, m.player.ID, fp); err != nil {
				m.deps.Log.Error("unlink key", "err", err)
			}
		}
	}
	if m.leave != nil {
		m.leave()
		m.leave = nil
	}
	m.player, m.name, m.color = nil, "", theme.DefaultPlayerColor
	m.onlineSince = time.Time{}
	m.lobby = newLobby()
	return tea.Batch(m.openWelcome(), m.notify("Logged out.", false))
}

// ---------------------------------------------------------------------------
// Yes/no questions

type confirmState struct {
	question, detail string
	yes, no          func() tea.Cmd
	back             screen
}

func (m *App) ask(question, detail string, yes, no func() tea.Cmd) tea.Cmd {
	m.confirm = confirmState{question: question, detail: detail, yes: yes, no: no, back: m.screen}
	m.screen = screenConfirm
	return nil
}

func (m *App) updateConfirm(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "y", "enter":
		return m.confirm.yes()
	case "n", "esc":
		if m.confirm.no != nil {
			return m.confirm.no()
		}
		m.screen = m.confirm.back
	}
	return nil
}

func (m *App) viewConfirm() string {
	t := m.th
	box := t.Modal.Width(52).Render(lipgloss.JoinVertical(lipgloss.Left,
		t.Title.Render(m.confirm.question),
		"",
		t.Base.Width(44).Render(m.confirm.detail),
		"",
		m.hints("y", "yes", "n", "no"),
	))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// shortFingerprint abbreviates "SHA256:abcdef..." for display.
func shortFingerprint(fp string) string {
	const keep = 12
	fp = strings.TrimPrefix(fp, "SHA256:")
	if len(fp) > keep {
		fp = fp[:keep] + "…"
	}
	return "SHA256:" + fp
}
