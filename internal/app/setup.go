package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/store"
	"termcade/internal/textutil"
	"termcade/internal/ui/theme"
)

// setupForm is used both for first-time onboarding and for editing the
// profile later from the lobby.
type setupForm struct {
	editing bool // true when opened from the lobby
	field   int  // 0 = name, 1 = color
	name    textinput.Model
	color   int // index into theme.PlayerColors
	err     string
}

const (
	fieldName = iota
	fieldColor
)

func (m *App) openSetup(editing bool) tea.Cmd {
	t := m.th
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = textutil.NameMaxLen
	in.Width = textutil.NameMaxLen + 1
	in.Placeholder = "your nickname"
	in.TextStyle = t.Bold
	in.PlaceholderStyle = t.Faded
	in.Cursor.Style = t.Fg(theme.Pink)
	in.Cursor.TextStyle = t.Bold

	name := m.name
	if name == "" {
		name = textutil.SuggestName(m.id.User)
		if name != "" && m.nameUnavailable(name) != nil {
			name = ""
		}
	}
	in.SetValue(name)
	in.CursorEnd()

	color := 0
	for i, c := range theme.PlayerColors {
		if c.Key == m.color {
			color = i
		}
	}
	m.setup = setupForm{editing: editing, name: in, color: color}
	m.screen = screenSetup
	return m.setup.name.Focus()
}

func (m *App) updateSetup(msg tea.Msg) tea.Cmd {
	f := &m.setup
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		var cmd tea.Cmd
		f.name, cmd = f.name.Update(msg)
		return cmd
	}

	switch key.String() {
	case "esc":
		if f.editing {
			return m.enterLobby()
		}
		return nil
	case "enter":
		return m.submitSetup()
	case "tab", "shift+tab", "down", "up":
		if f.field == fieldName {
			f.field = fieldColor
			f.name.Blur()
			return nil
		}
		f.field = fieldName
		return f.name.Focus()
	}

	if f.field == fieldColor {
		n := len(theme.PlayerColors)
		switch key.String() {
		case "left", "h":
			f.color = (f.color + n - 1) % n
		case "right", "l", " ":
			f.color = (f.color + 1) % n
		}
		return nil
	}

	var cmd tea.Cmd
	f.name, cmd = f.name.Update(msg)
	f.err = ""
	return cmd
}

// nameUnavailable returns why name can't be used by this session, if at all.
func (m *App) nameUnavailable(name string) error {
	if err := textutil.ValidateName(name); err != nil {
		return err
	}
	if strings.EqualFold(name, m.name) {
		return nil // keeping your own name is always fine
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	taken, err := m.deps.Store.NameTaken(ctx, name, m.playerID())
	if err != nil {
		m.deps.Log.Error("check name", "err", err)
		return errors.New("something went wrong, try again")
	}
	if taken || m.deps.Hub.NameOnline(name, m.id.SessionID) {
		return store.ErrNameTaken
	}
	return nil
}

func (m *App) submitSetup() tea.Cmd {
	f := &m.setup
	name := strings.TrimSpace(f.name.Value())
	color := theme.PlayerColors[f.color].Key
	if err := m.nameUnavailable(name); err != nil {
		f.err = err.Error()
		f.field = fieldName
		return f.name.Focus()
	}

	if !m.id.Guest() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var err error
		if m.player == nil {
			var p *store.Player
			p, err = m.deps.Store.CreatePlayer(ctx, m.id.Fingerprint, name, color)
			if err == nil {
				m.player = p
			}
		} else if err = m.deps.Store.UpdateProfile(ctx, m.player.ID, name, color); err == nil {
			m.player.Name, m.player.Color = name, color
		}
		if err != nil {
			if !errors.Is(err, store.ErrNameTaken) {
				m.deps.Log.Error("save profile", "err", err)
				err = errors.New("couldn't save your profile, try again")
			}
			f.err = err.Error()
			return nil
		}
	}

	changed := name != m.name || color != m.color
	first := m.name == ""
	m.name, m.color = name, color
	if m.leave != nil && changed {
		m.deps.Hub.Rename(m.id.SessionID, name, color)
	}
	cmd := m.enterLobby()
	switch {
	case first:
		return tea.Batch(cmd, m.notify("Welcome to the arcade, "+name+"!", false))
	case changed:
		return tea.Batch(cmd, m.notify("Profile saved.", false))
	}
	return cmd
}

func (m *App) viewSetup() string {
	t := m.th
	f := &m.setup
	const formW = 50

	heading := "CREATE YOUR PLAYER"
	if f.editing {
		heading = "EDIT PROFILE"
	}

	label := func(text string, active bool) string {
		if active {
			return t.Fg(theme.Pink).Bold(true).Render("▸ " + text)
		}
		return t.Dim.Bold(true).Render("  " + text)
	}

	// Name field.
	nameLine := "  " + f.name.View()
	nameHelp := t.Faded.Render("  3–16 letters, digits, - or _")
	if f.err != "" {
		nameHelp = t.Error.Render("  ✗ " + f.err)
	}

	// Color swatches.
	var sw strings.Builder
	sw.WriteString(" ")
	for i, c := range theme.PlayerColors {
		dot := t.Fg(c.Hex).Render("●")
		if i == f.color {
			bracket := t.Faded
			if f.field == fieldColor {
				bracket = t.Bold
			}
			dot = bracket.Render("[") + t.Fg(c.Hex).Render("●") + bracket.Render("]")
		} else {
			dot = " " + dot + " "
		}
		sw.WriteString(dot)
	}
	colorName := t.Fg(theme.PlayerColors[f.color].Hex).Render("  " + theme.PlayerColors[f.color].Key)

	// Live preview of how you'll appear in chat.
	previewName := strings.TrimSpace(f.name.Value())
	if previewName == "" {
		previewName = "you"
	}
	preview := t.Faded.Render("  now ") +
		t.PlayerName(previewName, theme.PlayerColors[f.color].Key) +
		t.Base.Render(" hello, arcade!")

	form := lipgloss.JoinVertical(lipgloss.Left,
		t.Title.Render(heading),
		"",
		label("NAME", f.field == fieldName),
		nameLine,
		nameHelp,
		"",
		label("COLOR", f.field == fieldColor),
		sw.String(),
		colorName,
		"",
		t.Dim.Bold(true).Render("  PREVIEW"),
		preview,
	)
	box := t.Modal.Width(formW).Render(form)

	var footer []string
	if f.editing {
		footer = append(footer, m.hints("tab", "switch field", "←→", "color", "enter", "save", "esc", "cancel"))
	} else {
		footer = append(footer, m.hints("tab", "switch field", "←→", "color", "enter", "start playing"))
	}
	if m.id.Guest() {
		footer = append(footer, "",
			t.Fg(theme.Amber).Render("◆ playing as a guest")+
				t.Faded.Render(" · connect with an SSH key to keep your name"))
	} else {
		footer = append(footer, "", t.Faded.Render("◆ your profile is tied to key "+shortFingerprint(m.id.Fingerprint)))
	}

	logo := theme.BigLogo(theme.Name)
	if m.height < 36 || lipgloss.Width(logo) > m.width-4 {
		logo = theme.SmallLogo()
	}
	art := t.Gradient(logo, theme.LogoGradient, lipgloss.Width(logo), 0, true)

	body := lipgloss.JoinVertical(lipgloss.Center,
		art, t.Dim.Render(theme.Tagline), "", box, "", lipgloss.JoinVertical(lipgloss.Center, footer...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
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
