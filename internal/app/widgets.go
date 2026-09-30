package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termcade/internal/ui/theme"
)

// fit truncates or pads a styled string to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// spread places left and right at the two ends of a w-cell line.
func spread(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fit(left, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

// panel draws a rounded box of exactly w×h cells with a title set into the
// top border. Content lines are clipped or padded to fit inside a one-cell
// horizontal padding.
func (m *App) panel(title, content string, w, h int, focused bool) string {
	t := m.th
	border := t.R.NewStyle().Foreground(t.Border)
	titleStyle := t.Dim.Bold(true)
	if focused {
		border = t.Fg(theme.Violet)
		titleStyle = t.Fg(theme.Pink).Bold(true)
	}
	innerW := w - 4
	innerH := h - 2
	if innerW < 1 || innerH < 0 {
		return ""
	}

	var b strings.Builder
	// Top border with the title: ╭─ TITLE ─────╮
	label := titleStyle.Render(title)
	rest := w - 5 - lipgloss.Width(label)
	if rest < 0 {
		label = fit(label, w-5)
		rest = 0
	}
	b.WriteString(border.Render("╭─ "))
	b.WriteString(label)
	b.WriteString(border.Render(" " + strings.Repeat("─", rest) + "╮"))
	b.WriteByte('\n')

	lines := strings.Split(content, "\n")
	side := border.Render("│")
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		b.WriteString(side + " " + fit(line, innerW) + " " + side + "\n")
	}
	b.WriteString(border.Render("╰" + strings.Repeat("─", w-2) + "╯"))
	return b.String()
}

// hint renders a key hint line such as "enter play  q quit" from key/label
// pairs.
func (m *App) hints(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, m.th.Key.Render(pairs[i])+" "+m.th.Faded.Render(pairs[i+1]))
	}
	return strings.Join(parts, m.th.Faded.Render("  ·  "))
}

// ago formats the time since t compactly: "now", "5m", "3h", "2d".
func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// wrap word-wraps plain text to width w, breaking words that are too long,
// and returns the lines.
func wrap(text string, w int) []string {
	if w < 1 {
		return nil
	}
	return strings.Split(ansi.Wrap(text, w, ""), "\n")
}

// center horizontally centers every line of s within w cells as a block,
// keeping the lines left-aligned relative to each other.
func center(s string, w int) string {
	return lipgloss.PlaceHorizontal(w, lipgloss.Center, s)
}
