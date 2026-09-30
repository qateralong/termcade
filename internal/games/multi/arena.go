package multi

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termcade/internal/ui/canvas"
	"termcade/internal/ui/layout"
	"termcade/internal/ui/theme"
)

// Standard play area for realtime arena games: a 60×36 pixel canvas (60×18
// characters) with a border, next to a 16-column sidebar. The whole thing is
// 79×20, which fits an 80×24 terminal with the header and footer.
const (
	ArenaW   = 60
	ArenaH   = 36
	SidebarW = 16
)

// SidebarRow is a player line in the sidebar.
type SidebarRow struct {
	Name  string
	Color string
	Value string
	You   bool
	Out   bool // eliminated or waiting to respawn
}

// Arena frames a canvas and a sidebar with a title, a big status value (such
// as time left) and player rows.
func Arena(t *theme.Theme, c *canvas.Canvas, title, status string, rows []SidebarRow, notes ...string) string {
	board := t.Panel.BorderForeground(lipgloss.Color(theme.Violet)).
		Render(c.Render(t.R.ColorProfile()))
	h := lipgloss.Height(board)

	inner := SidebarW - 2
	var lines []string
	lines = append(lines, t.Dim.Bold(true).Render(title))
	if status != "" {
		lines = append(lines, t.Bold.Render(status))
	}
	lines = append(lines, "")
	for _, r := range rows {
		mark := "●"
		style := t.Fg(r.Color).Bold(true)
		if r.Out {
			mark = "○"
			style = t.Fg(r.Color).Faint(true)
		}
		name := r.Name
		if len([]rune(name)) > 8 {
			name = string([]rune(name)[:8])
		}
		left := style.Render(mark + " " + name)
		if r.You {
			left += t.Faded.Render("*")
		}
		lines = append(lines, layout.Spread(left, t.Bold.Render(r.Value), inner))
	}
	if len(notes) > 0 {
		lines = append(lines, "")
		for _, n := range notes {
			lines = append(lines, t.Faded.Render(n))
		}
	}
	for len(lines) < h-2 {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, inner, "")
	}
	side := t.Panel.Width(inner).Height(h - 2).Render(strings.Join(lines[:h-2], "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, board, " ", side)
}

// Clock formats a duration as m:ss.
func Clock(secs int) string {
	if secs < 0 {
		secs = 0
	}
	s := strconv.Itoa(secs % 60)
	if len(s) < 2 {
		s = "0" + s
	}
	return strconv.Itoa(secs/60) + ":" + s
}
