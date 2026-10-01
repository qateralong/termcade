package battleship

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/ui/layout"
	"termcade/internal/ui/theme"
)

// ViewW and ViewH are the fixed size of the play area.
const (
	ViewW = 78
	ViewH = 20
)

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	me, enemy := w.p[seat], w.p[1-seat]
	aiming := w.phase == battle && w.turn == seat

	left := w.grid(th, me.board, false, false, 0, 0, w.last[seat], w.hasLast[seat])
	right := w.grid(th, enemy.board, true, aiming, me.cx, me.cy, w.last[1-seat], w.hasLast[1-seat])

	title := func(s, sub string, on bool) string {
		st := th.Dim.Bold(true)
		if on {
			st = th.Title
		}
		return st.Render(s) + th.Faded.Render("  "+sub)
	}
	leftT := title("YOUR FLEET", strconv.Itoa(me.board.shipsLeft())+" afloat", w.phase == battle && w.turn != seat)
	rightT := title(strings.ToUpper(enemy.info.Name)+"'S WATERS", strconv.Itoa(enemy.board.shipsLeft())+" afloat", aiming)

	grids := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, leftT, left),
		"     ",
		lipgloss.JoinVertical(lipgloss.Left, rightT, right),
	)

	// Status line.
	var status string
	switch w.phase {
	case placing:
		left := int((placeTime - w.clock + time.Second - 1) / time.Second)
		if me.ready {
			status = th.Success.Render("✓ Ready.") + th.Faded.Render(" Waiting for "+enemy.info.Name+"… "+strconv.Itoa(left)+"s")
		} else {
			status = th.Fg(theme.Amber).Bold(true).Render("Arrange your fleet: ") +
				th.Key.Render("r") + th.Faded.Render(" reshuffle  ·  ") + th.Key.Render("enter") +
				th.Faded.Render(" ready  ·  "+strconv.Itoa(left)+"s")
		}
	case battle:
		left := int((turnTime - (w.clock - w.phaseAt) + time.Second - 1) / time.Second)
		if aiming {
			status = th.Fg(theme.Lime).Bold(true).Render("▶ Your shot! ") +
				th.Faded.Render("aim with the arrows, fire with enter  ·  ") + timer(th, left)
		} else {
			status = th.Dim.Render(enemy.info.Name+" is aiming…  ") + timer(th, left)
		}
	case over:
		if w.winner == seat {
			status = th.Fg(theme.Amber).Bold(true).Render("★ Their fleet is at the bottom of the sea!")
		} else {
			status = th.Error.Bold(true).Render("Your fleet is sunk.")
		}
	}

	var logLines []string
	for _, l := range w.log {
		logLines = append(logLines, th.Faded.Render("· ")+th.Dim.Render(l))
	}
	for len(logLines) < 3 {
		logLines = append([]string{""}, logLines...)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		grids,
		"",
		status,
		"",
		strings.Join(logLines, "\n"),
	)
	return lipgloss.Place(ViewW, ViewH, lipgloss.Center, lipgloss.Top, body)
}

func timer(th *theme.Theme, secs int) string {
	st := th.Dim
	if secs <= 5 {
		st = th.Error.Bold(true)
	}
	return st.Render(strconv.Itoa(max(0, secs)) + "s")
}

// grid draws a board. hidden boards show only what the opponent knows.
func (w *World) grid(th *theme.Theme, b *board, hidden, cursor bool, cx, cy int, last cell, hasLast bool) string {
	var sb strings.Builder
	sb.WriteString("    ")
	for x := 0; x < N; x++ {
		sb.WriteString(th.Faded.Render(colName(x) + " "))
	}
	border := th.Fg(theme.Violet)
	sb.WriteString("\n   " + border.Render("╭"+strings.Repeat("─", N*2+1)+"╮"))
	for y := 0; y < N; y++ {
		sb.WriteString("\n" + th.Faded.Render(layout.PadLeft(strconv.Itoa(y+1), 2)) + " " + border.Render("│") + " ")
		for x := 0; x < N; x++ {
			v := b.grid[y][x]
			if hidden {
				v = b.known(x, y)
			}
			var st lipgloss.Style
			var g string
			switch v {
			case water:
				st, g = th.Fg("#2E4A7A"), "· "
			case ship:
				st, g = th.Fg(theme.Sky), "██"
			case miss:
				st, g = th.Fg("#5A6E99"), "• "
			case hit:
				st, g = th.Fg(theme.Coral).Bold(true), "╳ "
			case sunk:
				st, g = th.Fg("#8E3B46"), "▓▓"
			}
			if hasLast && last.x == x && last.y == y && v != water {
				st = st.Background(lipgloss.Color("#3A2F55"))
			}
			if cursor && x == cx && y == cy {
				st = st.Background(lipgloss.Color(theme.Pink)).Foreground(lipgloss.Color("#1A1325"))
				if v == water {
					g = "◎ "
				}
			}
			// Keep the cell exactly two cells wide, drawing the
			// background across both.
			sb.WriteString(st.Render(g))
		}
		sb.WriteString(border.Render("│"))
	}
	sb.WriteString("\n   " + border.Render("╰"+strings.Repeat("─", N*2+1)+"╯"))
	return sb.String()
}
