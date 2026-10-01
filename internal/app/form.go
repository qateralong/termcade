package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/textutil"
	"termcade/internal/ui/theme"
)

type fieldKind int

const (
	textField fieldKind = iota
	passwordField
	colorField
)

// field is one input of a form.
type field struct {
	label string
	help  string
	kind  fieldKind
	input textinput.Model
	color int // index into theme.PlayerColors, for color fields
}

// form is a small vertical form: labeled fields, a cursor moving between
// them with tab and the arrows, and an error line.
type form struct {
	title  string
	intro  string // a line under the title
	fields []*field
	focus  int
	err    string
	submit string // label for the enter key in the hints
}

func (m *App) newInput(placeholder string, limit int, secret bool) textinput.Model {
	t := m.th
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = limit
	in.Width = limit + 1
	in.Placeholder = placeholder
	in.TextStyle = t.Bold
	in.PlaceholderStyle = t.Faded
	in.Cursor.Style = t.Fg(theme.Pink)
	in.Cursor.TextStyle = t.Bold
	if secret {
		in.EchoMode = textinput.EchoPassword
		in.EchoCharacter = '•'
	}
	return in
}

func (m *App) nameField(value string) *field {
	in := m.newInput("your nickname", textutil.NameMaxLen, false)
	in.SetValue(value)
	in.CursorEnd()
	return &field{label: "NAME", help: "3–16 letters, digits, - or _", kind: textField, input: in}
}

func (m *App) passwordField(label, help string) *field {
	return &field{label: label, help: help, kind: passwordField, input: m.newInput("••••••", 72, true)}
}

func (m *App) colorField(key string) *field {
	idx := 0
	for i, c := range theme.PlayerColors {
		if c.Key == key {
			idx = i
		}
	}
	return &field{label: "COLOR", kind: colorField, color: idx}
}

// value returns the text of field i.
func (f *form) value(i int) string { return f.fields[i].input.Value() }

// colorKey returns the color picked in the form's color field.
func (f *form) colorKey() string {
	for _, fl := range f.fields {
		if fl.kind == colorField {
			return theme.PlayerColors[fl.color].Key
		}
	}
	return theme.DefaultPlayerColor
}

// focusOn moves the cursor to field i.
func (f *form) focusOn(i int) tea.Cmd {
	for j, fl := range f.fields {
		if fl.kind != colorField {
			fl.input.Blur()
		}
		_ = j
	}
	f.focus = (i + len(f.fields)) % len(f.fields)
	if fl := f.fields[f.focus]; fl.kind != colorField {
		return fl.input.Focus()
	}
	return nil
}

// fail shows an error and puts the cursor on field i.
func (f *form) fail(i int, err error) tea.Cmd {
	f.err = err.Error()
	if i >= 0 {
		return f.focusOn(i)
	}
	return nil
}

// update handles a message. It reports whether the form was submitted
// (enter) or cancelled (esc).
func (f *form) update(msg tea.Msg) (submit, cancel bool, cmd tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	cur := f.fields[f.focus]
	if !ok {
		if cur.kind != colorField {
			cur.input, cmd = cur.input.Update(msg)
		}
		return false, false, cmd
	}
	switch key.String() {
	case "esc":
		return false, true, nil
	case "enter":
		// Enter on any field but the last moves on; on the last it submits.
		if f.focus < len(f.fields)-1 && cur.kind != colorField && cur.input.Value() == "" {
			return false, false, f.focusOn(f.focus + 1)
		}
		return true, false, nil
	case "tab", "down":
		return false, false, f.focusOn(f.focus + 1)
	case "shift+tab", "up":
		return false, false, f.focusOn(f.focus - 1)
	}
	if cur.kind == colorField {
		n := len(theme.PlayerColors)
		switch key.String() {
		case "left", "h":
			cur.color = (cur.color + n - 1) % n
		case "right", "l", " ":
			cur.color = (cur.color + 1) % n
		}
		return false, false, nil
	}
	cur.input, cmd = cur.input.Update(msg)
	f.err = ""
	return false, false, cmd
}

// viewForm draws a form in a centered card with the logo above it and
// hints and notes below.
func (m *App) viewForm(f *form, notes ...string) string {
	t := m.th
	const formW = 50

	label := func(text string, active bool) string {
		if active {
			return t.Fg(theme.Pink).Bold(true).Render("▸ " + text)
		}
		return t.Dim.Bold(true).Render("  " + text)
	}

	// Spread out in a tall terminal; stay compact in an 80×24 one.
	roomy := m.height >= 34

	lines := []string{t.Title.Render(f.title)}
	if f.intro != "" {
		lines = append(lines, t.Faded.Render(f.intro))
	}
	for i, fl := range f.fields {
		active := i == f.focus
		if roomy || i == 0 {
			lines = append(lines, "")
		}
		lines = append(lines, label(fl.label, active))
		switch fl.kind {
		case colorField:
			var sw strings.Builder
			sw.WriteString(" ")
			for ci, c := range theme.PlayerColors {
				if ci == fl.color {
					br := t.Faded
					if active {
						br = t.Bold
					}
					sw.WriteString(br.Render("[") + t.Fg(c.Hex).Render("●") + br.Render("]"))
				} else {
					sw.WriteString(" " + t.Fg(c.Hex).Render("●") + " ")
				}
			}
			lines = append(lines, sw.String()+t.Fg(theme.PlayerColors[fl.color].Hex).Render("  "+theme.PlayerColors[fl.color].Key))
		default:
			lines = append(lines, "  "+fl.input.View())
			if fl.help != "" && active {
				lines = append(lines, t.Faded.Render("  "+fl.help))
			}
		}
	}
	if f.err != "" {
		lines = append(lines, "", t.Error.Render("✗ "+f.err))
	}
	box := t.Modal.Width(formW).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	submit := f.submit
	if submit == "" {
		submit = "continue"
	}
	footer := []string{m.hints("tab", "next field", "enter", submit, "esc", "back")}
	for _, n := range notes {
		footer = append(footer, n)
	}

	parts := []string{box, "", lipgloss.JoinVertical(lipgloss.Center, footer...)}
	if roomy {
		logo := theme.SmallLogo()
		parts = append([]string{t.Gradient(logo, theme.LogoGradient, lipgloss.Width(logo), 0, true), ""}, parts...)
	}
	body := lipgloss.JoinVertical(lipgloss.Center, parts...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}
