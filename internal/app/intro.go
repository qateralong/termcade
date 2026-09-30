package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/ui/theme"
)

const (
	introFrame    = 40 * time.Millisecond
	introRevealed = 18 // frames until the logo is fully drawn
	introDuration = 75 // frames before moving on automatically
)

// intro is the splash screen: the logo sweeps in and its colors start to
// flow. Any key skips it.
type intro struct {
	frame int
	done  bool
}

type introTickMsg struct{}

func (intro) tick() tea.Cmd {
	return tea.Tick(introFrame, func(time.Time) tea.Msg { return introTickMsg{} })
}

func (i intro) init() tea.Cmd { return i.tick() }

func (m *App) updateIntro(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case introTickMsg:
		if m.intro.done {
			return nil
		}
		m.intro.frame++
		if m.intro.frame >= introDuration {
			m.intro.done = true
			return m.finishIntro()
		}
		return m.intro.tick()
	case tea.KeyMsg:
		if m.intro.done {
			return nil
		}
		m.intro.done = true
		return m.finishIntro()
	}
	return nil
}

func (m *App) viewIntro() string {
	t := m.th
	f := m.intro.frame
	phase := float64(f) * 0.012

	logo := theme.BigLogo(theme.Name)
	logoW := lipgloss.Width(logo)
	if logoW > m.width-4 {
		logo = theme.SmallLogo()
		logoW = lipgloss.Width(logo)
	}

	// Sweep the logo in from the left.
	visible := logoW
	if f < introRevealed {
		visible = logoW * f / introRevealed
	}
	lines := strings.Split(logo, "\n")
	for i, line := range lines {
		r := []rune(line)
		if visible < len(r) {
			lines[i] = string(r[:visible]) + strings.Repeat(" ", len(r)-visible)
		}
	}
	art := t.Gradient(strings.Join(lines, "\n"), theme.LogoGradient, logoW, phase, true)

	tagline := ""
	prompt := ""
	if f >= introRevealed {
		tagline = t.Dim.Render(typed(theme.Tagline, (f-introRevealed)*2))
		if (f/12)%2 == 0 {
			prompt = t.Faded.Render("press any key")
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Center, art, "", tagline, "", "", prompt)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// typed returns the first n runes of s, padded so the width stays constant
// and the centered text doesn't jump around while it is being typed.
func typed(s string, n int) string {
	r := []rune(s)
	if n >= len(r) {
		return s
	}
	if n < 0 {
		n = 0
	}
	return string(r[:n]) + strings.Repeat(" ", len(r)-n)
}
