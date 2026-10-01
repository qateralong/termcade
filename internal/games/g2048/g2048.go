// Package g2048 is the sliding-tile puzzle: merge equal tiles, reach 2048,
// and keep going for a high score.
package g2048

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

// N is the board size.
const N = 4

// Game returns 2048.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "2048",
			Name:        "2048",
			Icon:        "▩",
			Tagline:     "Slide, merge, double. Reach 2048, then keep going.",
			Description: "Every move slides all tiles as far as they go, and equal tiles that collide merge into one. A new tile appears after each move. Run out of moves and it's over.",
			Kind:        games.TurnBased,
			Players:     "1",
			Controls:    []string{"←↑→↓", "slide", "wasd", "slide", "u", "undo once"},
			Accent:      []string{theme.Amber, "#FF9F43", theme.Coral},
			Art: []string{
				"╭────┬────┬────┬────╮",
				"│  2 │  4 │    │  8 │",
				"│ 16 │ 32 │ 64 │    │",
				"╰────┴────┴────┴────╯",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 100 * time.Millisecond,
	})
}

// Engine is one game of 2048.
type Engine struct {
	th    *theme.Theme
	rng   *rand.Rand
	board [N][N]int
	score int
	best  int // best tile
	state solo.State

	prev      [N][N]int
	prevScore int
	canUndo   bool
	fresh     [2]int // the tile that just appeared, for highlighting
}

// New starts a game with two tiles.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, fresh: [2]int{-1, -1}}
	e.spawn()
	e.spawn()
	return e
}

func (e *Engine) spawn() {
	var empty [][2]int
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			if e.board[y][x] == 0 {
				empty = append(empty, [2]int{x, y})
			}
		}
	}
	if len(empty) == 0 {
		return
	}
	c := empty[e.rng.IntN(len(empty))]
	v := 2
	if e.rng.IntN(10) == 0 {
		v = 4
	}
	e.board[c[1]][c[0]] = v
	e.fresh = c
	e.best = max(e.best, v)
}

// slideRow slides one row toward index 0, merging equal neighbors once.
// It returns the new row and the points scored.
func slideRow(row [N]int) ([N]int, int) {
	var out [N]int
	pts, k := 0, 0
	merged := false
	for _, v := range row {
		if v == 0 {
			continue
		}
		if k > 0 && out[k-1] == v && !merged {
			out[k-1] *= 2
			pts += out[k-1]
			merged = true
			continue
		}
		out[k] = v
		k++
		merged = false
	}
	return out, pts
}

// move slides the whole board. dx/dy is the direction tiles move in.
func (e *Engine) move(dx, dy int) bool {
	before := e.board
	gained := 0
	for i := 0; i < N; i++ {
		var line [N]int
		for j := 0; j < N; j++ {
			x, y := e.cell(i, j, dx, dy)
			line[j] = e.board[y][x]
		}
		out, pts := slideRow(line)
		gained += pts
		for j := 0; j < N; j++ {
			x, y := e.cell(i, j, dx, dy)
			e.board[y][x] = out[j]
			e.best = max(e.best, out[j])
		}
	}
	if e.board == before {
		return false
	}
	e.prev, e.prevScore, e.canUndo = before, e.score, true
	e.score += gained
	e.spawn()
	if !e.movesLeft() {
		e.state = solo.Lost
	}
	return true
}

// cell maps line i, position j (0 = the edge tiles slide toward) to board
// coordinates for a move in direction dx/dy.
func (e *Engine) cell(i, j, dx, dy int) (int, int) {
	switch {
	case dx < 0:
		return j, i
	case dx > 0:
		return N - 1 - j, i
	case dy < 0:
		return i, j
	default:
		return i, N - 1 - j
	}
}

func (e *Engine) movesLeft() bool {
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			v := e.board[y][x]
			if v == 0 || (x+1 < N && e.board[y][x+1] == v) || (y+1 < N && e.board[y+1][x] == v) {
				return true
			}
		}
	}
	return false
}

// Update implements solo.Engine.
func (e *Engine) Update(time.Duration) {}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	switch k {
	case "left", "a", "h":
		e.move(-1, 0)
	case "right", "d", "l":
		e.move(1, 0)
	case "up", "w", "k":
		e.move(0, -1)
	case "down", "s", "j":
		e.move(0, 1)
	case "u", "backspace":
		if e.canUndo {
			e.board, e.score, e.canUndo = e.prev, e.prevScore, false
			e.fresh = [2]int{-1, -1}
		}
	}
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.score }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{{Label: "TILE", Value: strconv.Itoa(e.best)}}
}

// Tile colors from small to large.
var tileColors = map[int][2]string{
	2: {"#3A3452", "#E8E3F7"}, 4: {"#4A3F6B", "#E8E3F7"}, 8: {"#FF9F43", "#1A1325"},
	16: {"#FF7A6B", "#1A1325"}, 32: {"#FF4F5E", "#FFFFFF"}, 64: {"#E0263D", "#FFFFFF"},
	128: {"#FFC940", "#1A1325"}, 256: {"#FFD23F", "#1A1325"}, 512: {"#B8FF5A", "#1A1325"},
	1024: {"#4DFFC3", "#1A1325"}, 2048: {"#3DE8FF", "#1A1325"},
}

const tileW = 7

// View implements solo.Engine.
func (e *Engine) View() string {
	t := e.th
	border := t.Fg(theme.Violet)
	var sb strings.Builder
	sb.WriteString(border.Render("╭" + strings.Repeat(strings.Repeat("─", tileW)+"┬", N-1) + strings.Repeat("─", tileW) + "╮"))
	for y := 0; y < N; y++ {
		for line := 0; line < 3; line++ {
			sb.WriteString("\n" + border.Render("│"))
			for x := 0; x < N; x++ {
				v := e.board[y][x]
				cell := strings.Repeat(" ", tileW)
				if v > 0 {
					c, ok := tileColors[v]
					if !ok {
						c = [2]string{"#9D7BFF", "#FFFFFF"}
					}
					st := t.R.NewStyle().Background(lipgloss.Color(c[0])).Foreground(lipgloss.Color(c[1])).Bold(true)
					text := ""
					if line == 1 {
						text = strconv.Itoa(v)
					}
					if line == 0 && e.fresh == [2]int{x, y} {
						text = "·"
					}
					cell = st.Render(center(text, tileW))
				} else if line == 1 {
					cell = t.Faded.Render(center("·", tileW))
				}
				sb.WriteString(cell)
				if x < N-1 {
					sb.WriteString(border.Render("│"))
				}
			}
			sb.WriteString(border.Render("│"))
		}
		if y < N-1 {
			sb.WriteString("\n" + border.Render("├"+strings.Repeat(strings.Repeat("─", tileW)+"┼", N-1)+strings.Repeat("─", tileW)+"┤"))
		}
	}
	sb.WriteString("\n" + border.Render("╰"+strings.Repeat(strings.Repeat("─", tileW)+"┴", N-1)+strings.Repeat("─", tileW)+"╯"))
	return sb.String()
}

func center(s string, w int) string {
	n := len([]rune(s))
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}
