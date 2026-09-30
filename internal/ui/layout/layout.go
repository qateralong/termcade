// Package layout has small helpers for composing styled terminal text.
package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termcade/internal/ui/theme"
)

// Overlay draws fg centered on top of bg. If fg doesn't fit, it is returned
// on its own.
func Overlay(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	bw := lipgloss.Width(bg)
	fw := lipgloss.Width(fg)
	if fw > bw || len(fgLines) > len(bgLines) {
		return fg
	}
	x := (bw - fw) / 2
	y := (len(bgLines) - len(fgLines)) / 2
	for i, fl := range fgLines {
		line := bgLines[y+i]
		if w := ansi.StringWidth(line); w < bw {
			line += strings.Repeat(" ", bw-w)
		}
		left := ansi.Truncate(line, x, "")
		right := ansi.Cut(line, x+ansi.StringWidth(fl), bw)
		bgLines[y+i] = left + "\x1b[0m" + fl + "\x1b[0m" + right
	}
	return strings.Join(bgLines, "\n")
}

// PadRight pads s with spaces to w cells.
func PadRight(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// PadLeft pads s on the left with spaces to w cells.
func PadLeft(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// Spaced puts a space between every character: "ABC" -> "A B C".
func Spaced(s string) string {
	r := []rune(s)
	out := make([]string, len(r))
	for i, c := range r {
		out[i] = string(c)
	}
	return strings.Join(out, " ")
}

// KeyLine renders key hints from key/description pairs.
func KeyLine(t *theme.Theme, pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, t.Key.Render(pairs[i])+" "+t.Faded.Render(pairs[i+1]))
	}
	return strings.Join(parts, t.Faded.Render("  ·  "))
}

// Spread places left and right at the two ends of a w-cell line.
func Spread(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left+" "+right, w, "")
	}
	return left + strings.Repeat(" ", gap) + right
}

// Frame stacks a header, a body centered in the remaining space, and a
// footer into a w×h screen. It returns the screen and the body's top-left
// corner.
func Frame(header, body, footer string, w, h int) (string, int, int) {
	bodyW, bodyH := lipgloss.Width(body), lipgloss.Height(body)
	space := h - 2
	top := max(0, (space-bodyH)/2)
	left := max(0, (w-bodyW)/2)

	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	lines := strings.Split(body, "\n")
	pad := strings.Repeat(" ", left)
	for i := 0; i < space; i++ {
		if j := i - top; j >= 0 && j < len(lines) {
			b.WriteString(pad + lines[j])
		}
		b.WriteByte('\n')
	}
	b.WriteString(footer)
	return b.String(), left, 1 + top
}
