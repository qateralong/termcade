package bossrush

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/ui/layout"
	"termcade/internal/ui/theme"
)

// ViewW and ViewH are the fixed size of the play area.
const (
	ViewW = BoxW + 2
	ViewH = 20
)

// View implements solo.Engine.
func (e *Engine) View() string {
	t := e.th
	b := e.boss()

	// The boss: art, name and HP bar.
	art := make([]string, len(b.art))
	shake := e.phase == dodge && e.att != nil && e.att.elapsed < 300*time.Millisecond
	for i, line := range b.art {
		if e.phase == beaten && (e.clock/(120*time.Millisecond))%2 == 0 {
			line = strings.Repeat(" ", len([]rune(line)))
		}
		if shake && i%2 == 0 {
			line = " " + line
		}
		art[i] = t.Fg(b.color).Render(line)
	}
	bar := hpBar(t, e.bossHP, b.hp, 12, b.color)
	name := t.Bold.Render(strings.ToUpper(b.name)) + "  " + bar
	// Show the last strike for a moment after it lands.
	if e.phase == dodge && e.att.elapsed < 1500*time.Millisecond {
		hit := t.Error.Bold(true).Render("  −" + strconv.Itoa(e.lastHit))
		switch e.lastNote {
		case "MISS":
			hit = t.Faded.Render("  MISS")
		case "CRITICAL!":
			hit += t.Fg(theme.Amber).Bold(true).Render(" CRIT!")
		}
		name += hit
	}
	top := lipgloss.JoinVertical(lipgloss.Center, append(art, name)...)

	var box string
	if e.phase == strike {
		box = e.viewStrike()
	} else {
		box = e.viewBox()
	}

	heart := t.Fg(theme.Coral).Bold(true).Render("♥ ")
	hp := heart + t.Bold.Render("HP ") + hpBar(t, e.hp, maxHP, 12, theme.Coral) +
		t.Faded.Render("  "+strconv.Itoa(e.hp)+"/"+strconv.Itoa(maxHP))

	var bottom string
	switch e.phase {
	case menu:
		fight := " FIGHT "
		heal := " HEAL ×" + strconv.Itoa(e.items) + " "
		sel := t.R.NewStyle().Background(lipgloss.Color(theme.Amber)).Foreground(lipgloss.Color("#1A1325")).Bold(true)
		off := t.Fg(theme.Amber)
		if e.menuSel == 0 {
			bottom = sel.Render("✦"+fight) + "   " + off.Render(" "+heal)
		} else {
			bottom = off.Render(" "+fight) + "   " + sel.Render("✦"+heal)
		}
	case strike:
		bottom = t.Key.Render("enter") + t.Faded.Render(" to strike!")
	case dodge:
		if e.blue {
			bottom = t.Fg(theme.Sky).Bold(true).Render("blue heart: ") + t.Faded.Render("↑ jumps")
		} else {
			left := (e.att.duration - e.att.elapsed + time.Second - 1) / time.Second
			bottom = t.Faded.Render("survive " + strconv.Itoa(int(left)) + "s")
		}
	default:
		bottom = t.Faded.Render("press ") + t.Key.Render("enter")
	}

	body := lipgloss.JoinVertical(lipgloss.Center, top, box, layout.PadRight(hp, ViewW), layout.PadRight(bottom, ViewW))
	return lipgloss.Place(ViewW, ViewH, lipgloss.Center, lipgloss.Top, body)
}

// viewBox draws the bullet box with the heart, or the narration text when
// the boss isn't attacking.
func (e *Engine) viewBox() string {
	t := e.th
	border := t.Fg("#FFFFFF")
	grid := make([][]string, BoxH)
	for y := range grid {
		grid[y] = make([]string, BoxW)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}

	if e.phase == dodge {
		for _, b := range e.beams {
			firing := e.att.elapsed >= b.fireAt
			if !firing && (e.clock/(100*time.Millisecond))%2 == 1 {
				continue
			}
			st := t.Fg(theme.Coral)
			glyph := "░"
			if firing {
				st, glyph = t.Fg("#FFFFFF").Bold(true), "█"
			}
			for i := 0; i < max(BoxW, BoxH); i++ {
				x, y := b.pos, i
				if b.horiz {
					x, y = i, b.pos
				}
				if x < BoxW && y < BoxH {
					grid[y][x] = st.Render(glyph)
				}
			}
		}
		for _, b := range e.bullets {
			bx, by := int(math.Floor(b.x)), int(math.Floor(b.y))
			for dy := 0; dy < b.h; dy++ {
				for dx := 0; dx < b.w; dx++ {
					x, y := bx+dx, by+dy
					if x >= 0 && y >= 0 && x < BoxW && y < BoxH {
						grid[y][x] = t.Fg(b.color).Bold(true).Render(b.glyph)
					}
				}
			}
		}
		hurt := e.clock-e.hurtAt < invulnTime
		if !hurt || (e.clock/(80*time.Millisecond))%2 == 0 {
			col := theme.Coral
			if e.blue {
				col = theme.Sky
			}
			grid[int(math.Round(e.hy))][int(math.Round(e.hx))] = t.Fg(col).Bold(true).Render("♥")
		}
	} else {
		// Narration, word-wrapped inside the box.
		text := "* " + e.line
		words := strings.Fields(text)
		row, col := 1, 1
		for _, w := range words {
			if col+len([]rune(w)) >= BoxW-1 {
				row, col = row+1, 3
			}
			if row >= BoxH {
				break
			}
			for _, r := range w {
				grid[row][col] = t.Base.Render(string(r))
				col++
			}
			col++
		}
	}

	var sb strings.Builder
	sb.WriteString(border.Render("┌" + strings.Repeat("─", BoxW) + "┐"))
	for _, row := range grid {
		sb.WriteString("\n" + border.Render("│") + strings.Join(row, "") + border.Render("│"))
	}
	sb.WriteString("\n" + border.Render("└"+strings.Repeat("─", BoxW)+"┘"))
	return sb.String()
}

// viewStrike draws the timing bar inside the box.
func (e *Engine) viewStrike() string {
	t := e.th
	border := t.Fg("#FFFFFF")
	inner := BoxW - 4
	var bar strings.Builder
	for i := 0; i < inner; i++ {
		d := math.Abs(float64(i)/float64(inner-1) - 0.5)
		col := "#3A3452"
		switch {
		case d < 0.04:
			col = theme.Amber
		case d < 0.2:
			col = theme.Lime
		case d < 0.35:
			col = "#2E7D3A"
		}
		bar.WriteString(t.Fg(col).Render("█"))
	}
	cursor := strings.Repeat(" ", int(e.barPos*float64(inner-1))) + t.Bold.Render("▼")
	lines := []string{
		"",
		t.Bold.Render("STRIKE!"),
		"",
		"  " + layout.PadRight(cursor, inner),
		"  " + bar.String(),
		"  " + bar.String(),
		"",
		t.Faded.Render("closer to the middle = harder hit"),
	}
	for len(lines) < BoxH {
		lines = append(lines, "")
	}
	var sb strings.Builder
	sb.WriteString(border.Render("┌" + strings.Repeat("─", BoxW) + "┐"))
	for _, l := range lines[:BoxH] {
		sb.WriteString("\n" + border.Render("│") + lipgloss.PlaceHorizontal(BoxW, lipgloss.Center, l) + border.Render("│"))
	}
	sb.WriteString("\n" + border.Render("└"+strings.Repeat("─", BoxW)+"┘"))
	return sb.String()
}

func hpBar(t *theme.Theme, v, maxV, width int, color string) string {
	filled := int(math.Ceil(float64(v) / float64(maxV) * float64(width)))
	filled = max(0, min(width, filled))
	return t.Fg(color).Render(strings.Repeat("█", filled)) + t.R.NewStyle().Foreground(lipgloss.Color("#3A3452")).Render(strings.Repeat("█", width-filled))
}
