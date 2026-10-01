package muncher

import (
	"strings"
	"time"

	"termcade/internal/ui/theme"
)

const (
	wallColor   = "#4D6BFF"
	pacColor    = "#FFD23F"
	scaredColor = "#3A5BFF"
	doorColor   = "#FF8BD8"
)

// View implements solo.Engine. Every cell is two characters wide, with its
// glyph in the first one, which keeps entities centered in the corridors.
func (e *Engine) View() string {
	t := e.th
	grid := make([][]string, MazeH)
	flashWalls := e.phase == phaseCleared && (e.phaseTime/(250*time.Millisecond))%2 == 1
	wall := t.Fg(wallColor)
	if flashWalls {
		wall = t.Fg("#FFFFFF")
	}
	dot := t.Fg("#F5C9B8")
	pelletOn := (e.blink/(200*time.Millisecond))%2 == 0 || e.phase != phasePlay

	for y := 0; y < MazeH; y++ {
		grid[y] = make([]string, MazeW)
		for x := 0; x < MazeW; x++ {
			switch {
			case e.mz.tiles[y][x] == tileWall:
				grid[y][x] = wall.Render(e.walls[y][x])
			case e.mz.tiles[y][x] == tileDoor:
				grid[y][x] = t.Fg(doorColor).Render("──")
			case e.mz.dots[y][x] == 1:
				grid[y][x] = dot.Render("· ")
			case e.mz.dots[y][x] == 2:
				if pelletOn {
					grid[y][x] = dot.Bold(true).Render("● ")
				} else {
					grid[y][x] = "  "
				}
			default:
				grid[y][x] = "  "
			}
		}
	}

	if e.fruit {
		grid[pacStart.y][pacStart.x] = t.Fg(theme.Pink).Bold(true).Render("◆ ")
	}

	if e.phase != phaseCleared {
		for _, g := range e.ghosts {
			if e.phase == phaseDying && e.phaseTime > dyingTime/3 {
				break // ghosts vanish while the muncher dies
			}
			grid[g.pos.y][g.pos.x] = e.ghostGlyph(g)
		}
	}
	grid[e.pac.pos.y][e.pac.pos.x] = e.pacGlyph()

	if e.phase == phaseReady {
		writeText(grid, readyRow, t.Fg(pacColor).Bold(true).Render, "READY!")
	}

	var sb strings.Builder
	for y, row := range grid {
		if y > 0 {
			sb.WriteByte('\n')
		}
		for _, c := range row {
			sb.WriteString(c)
		}
	}
	return sb.String()
}

func (e *Engine) pacGlyph() string {
	st := e.th.Fg(pacColor).Bold(true)
	if e.phase == phaseDying {
		// Shrink away.
		frames := []string{"● ", "◒ ", "◡ ", "· ", "  "}
		i := int(e.phaseTime * time.Duration(len(frames)) / dyingTime)
		return st.Render(frames[min(i, len(frames)-1)])
	}
	if !e.chew || e.phase != phasePlay {
		return st.Render("● ")
	}
	switch e.pac.dir {
	case dirRight:
		return st.Render("◖ ")
	case dirLeft:
		return st.Render("◗ ")
	case dirUp:
		return st.Render("◒ ")
	case dirDown:
		return st.Render("◓ ")
	}
	return st.Render("● ")
}

func (e *Engine) ghostGlyph(g *ghost) string {
	t := e.th
	switch {
	case g.mode == ghostEaten:
		return t.Fg("#FFFFFF").Bold(true).Render("ö ")
	case g.frightened:
		c := scaredColor
		if e.fright < 2*time.Second && (e.blink/(200*time.Millisecond))%2 == 0 {
			c = "#FFFFFF"
		}
		return t.Fg(c).Bold(true).Render("Ω ")
	}
	return t.Fg(g.color).Bold(true).Render("Ω ")
}

// writeText centers text on a maze row, one character per cell half.
func writeText(grid [][]string, row int, render func(...string) string, text string) {
	cells := (len(text) + 1) / 2
	start := (MazeW - cells) / 2
	padded := text + strings.Repeat(" ", cells*2-len(text))
	for i := 0; i < cells; i++ {
		grid[row][start+i] = render(padded[i*2 : i*2+2])
	}
}
