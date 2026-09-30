// Package minesweeper is the classic mine-clearing puzzle, playable with the
// keyboard or the mouse.
package minesweeper

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

// Difficulty is a board size and mine count.
type Difficulty struct {
	W, H, Mines int
}

// Difficulties by variant key.
var Difficulties = map[string]Difficulty{
	"easy":   {9, 9, 10},
	"medium": {16, 16, 40},
	"hard":   {30, 16, 99},
}

// Game returns the minesweeper game.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "minesweeper",
			Name:        "Minesweeper",
			Icon:        "✱",
			Tagline:     "Clear the field. Trust the numbers.",
			Description: "Each number tells you how many mines touch that square. Flag the mines, open everything else, and do it fast. Works with the mouse too.",
			Kind:        games.TurnBased,
			Players:     "1",
			Controls:    []string{"←↑→↓", "move", "space", "open", "f", "flag", "mouse", "L open / R flag"},
			Accent:      []string{theme.Sky, theme.Cyan, theme.Violet},
			Art: []string{
				"  1  1  2  ⚑  1",
				"  1  ⚑  3  2  1",
				"  1  2  ⚑  1",
				"     1  1  1",
			},
		},
		Variants: []solo.Variant{
			{Key: "easy", Name: "Easy", Desc: "9×9 · 10 mines"},
			{Key: "medium", Name: "Medium", Desc: "16×16 · 40 mines"},
			{Key: "hard", Name: "Hard", Desc: "30×16 · 99 mines"},
		},
		New: func(s games.Session, variant string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, Difficulties[variant], rng)
		},
		LowerIsBetter: true,
		OnlyWins:      true,
		ScoreLabel:    "TIME",
		FormatScore:   solo.Duration,
		Mouse:         true,
		Tick:          100 * time.Millisecond,
	})
}

type cell struct {
	mine, open, flag bool
	n                int // neighboring mines
}

// Engine is one minesweeper board.
type Engine struct {
	th    *theme.Theme
	rng   *rand.Rand
	d     Difficulty
	cells []cell

	cx, cy  int  // cursor
	started bool // mines are placed on the first open
	elapsed time.Duration
	opened  int
	flags   int
	boomX   int
	boomY   int
	state   solo.State
}

// New creates a board.
func New(th *theme.Theme, d Difficulty, rng *rand.Rand) *Engine {
	if d.W == 0 {
		d = Difficulties["easy"]
	}
	return &Engine{th: th, rng: rng, d: d, cells: make([]cell, d.W*d.H), cx: d.W / 2, cy: d.H / 2, boomX: -1}
}

func (e *Engine) at(x, y int) *cell { return &e.cells[y*e.d.W+x] }

func (e *Engine) in(x, y int) bool { return x >= 0 && y >= 0 && x < e.d.W && y < e.d.H }

func (e *Engine) neighbors(x, y int, f func(nx, ny int)) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && e.in(x+dx, y+dy) {
				f(x+dx, y+dy)
			}
		}
	}
}

// placeMines lays mines anywhere except around (sx, sy), so the first
// click always opens an area.
func (e *Engine) placeMines(sx, sy int) {
	safe := func(x, y int) bool { return abs(x-sx) <= 1 && abs(y-sy) <= 1 }
	var spots []int
	for y := 0; y < e.d.H; y++ {
		for x := 0; x < e.d.W; x++ {
			if !safe(x, y) {
				spots = append(spots, y*e.d.W+x)
			}
		}
	}
	e.rng.Shuffle(len(spots), func(i, j int) { spots[i], spots[j] = spots[j], spots[i] })
	for _, i := range spots[:min(e.d.Mines, len(spots))] {
		e.cells[i].mine = true
	}
	for y := 0; y < e.d.H; y++ {
		for x := 0; x < e.d.W; x++ {
			c := e.at(x, y)
			e.neighbors(x, y, func(nx, ny int) {
				if e.at(nx, ny).mine {
					c.n++
				}
			})
		}
	}
	e.started = true
}

// open reveals a cell; opening a revealed number whose flags are all placed
// opens its remaining neighbors ("chording").
func (e *Engine) open(x, y int) {
	if e.state != solo.Playing || !e.in(x, y) {
		return
	}
	if !e.started {
		e.placeMines(x, y)
	}
	c := e.at(x, y)
	if c.flag {
		return
	}
	if c.open {
		flags := 0
		e.neighbors(x, y, func(nx, ny int) {
			if e.at(nx, ny).flag {
				flags++
			}
		})
		if c.n > 0 && flags == c.n {
			e.neighbors(x, y, func(nx, ny int) {
				if n := e.at(nx, ny); !n.open && !n.flag {
					e.reveal(nx, ny)
				}
			})
		}
	} else {
		e.reveal(x, y)
	}
	e.checkWin()
}

// reveal opens a closed cell, flooding outward from empty ones.
func (e *Engine) reveal(x, y int) {
	stack := [][2]int{{x, y}}
	for len(stack) > 0 && e.state == solo.Playing {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		c := e.at(p[0], p[1])
		if c.open || c.flag {
			continue
		}
		c.open = true
		if c.mine {
			e.boomX, e.boomY = p[0], p[1]
			e.state = solo.Lost
			return
		}
		e.opened++
		if c.n == 0 {
			e.neighbors(p[0], p[1], func(nx, ny int) { stack = append(stack, [2]int{nx, ny}) })
		}
	}
}

func (e *Engine) checkWin() {
	if e.state == solo.Playing && e.opened == e.d.W*e.d.H-e.d.Mines {
		e.state = solo.Won
		for i := range e.cells {
			if e.cells[i].mine && !e.cells[i].flag {
				e.cells[i].flag = true
				e.flags++
			}
		}
	}
}

func (e *Engine) toggleFlag(x, y int) {
	if !e.in(x, y) || !e.started {
		return
	}
	c := e.at(x, y)
	if c.open {
		return
	}
	c.flag = !c.flag
	if c.flag {
		e.flags++
	} else {
		e.flags--
	}
}

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	if e.started && e.state == solo.Playing {
		e.elapsed += dt
	}
}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	switch k {
	case "up", "w", "k":
		e.cy = max(0, e.cy-1)
	case "down", "s", "j":
		e.cy = min(e.d.H-1, e.cy+1)
	case "left", "a", "h":
		e.cx = max(0, e.cx-1)
	case "right", "d", "l":
		e.cx = min(e.d.W-1, e.cx+1)
	case " ", "enter", "o":
		e.open(e.cx, e.cy)
	case "f", "x", "m":
		e.toggleFlag(e.cx, e.cy)
	}
}

// Mouse implements solo.MouseEngine. The board has a one-cell border and
// each square is two characters wide.
func (e *Engine) Mouse(x, y int, b tea.MouseButton) {
	cx, cy := (x-1)/2, y-1
	if x < 1 || !e.in(cx, cy) {
		return
	}
	e.cx, e.cy = cx, cy
	switch b {
	case tea.MouseButtonLeft:
		e.open(cx, cy)
	case tea.MouseButtonRight, tea.MouseButtonMiddle:
		e.toggleFlag(cx, cy)
	}
}

// Score implements solo.Engine: the time taken, in milliseconds.
func (e *Engine) Score() int { return int(e.elapsed / time.Millisecond) }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{{Label: "MINES", Value: strconv.Itoa(e.d.Mines - e.flags)}}
}

var numberColors = [9]string{"", theme.Sky, theme.Lime, theme.Coral, theme.Violet, theme.Amber, theme.Cyan, theme.Pink, "#FFFFFF"}

// View implements solo.Engine.
func (e *Engine) View() string {
	t := e.th
	border := t.Fg(theme.Violet)
	dark := lipgloss.Color("#2B2440")
	light := lipgloss.Color("#342C4D")
	cursorBg := lipgloss.Color(theme.Pink)
	ink := lipgloss.Color("#1A1325")

	var sb strings.Builder
	sb.WriteString(border.Render("╭" + strings.Repeat("─", e.d.W*2) + "╮"))
	sb.WriteByte('\n')
	for y := 0; y < e.d.H; y++ {
		sb.WriteString(border.Render("│"))
		for x := 0; x < e.d.W; x++ {
			c := e.at(x, y)
			over := e.state != solo.Playing
			cursor := x == e.cx && y == e.cy && !over

			st := t.R.NewStyle()
			text := "  "
			switch {
			case c.open && c.mine:
				st = st.Background(lipgloss.Color(theme.Coral)).Foreground(ink).Bold(true)
				text = "✱ "
			case c.open && c.n > 0:
				st = st.Foreground(lipgloss.Color(numberColors[c.n])).Bold(true)
				text = strconv.Itoa(c.n) + " "
			case c.open:
				text = "  "
			case c.flag:
				bg := dark
				if (x+y)%2 == 1 {
					bg = light
				}
				st = st.Background(bg).Foreground(lipgloss.Color(theme.Coral)).Bold(true)
				text = "⚑ "
				if over && !c.mine { // wrong flag
					text = "╳ "
				}
			case over && c.mine:
				st = st.Background(dark).Foreground(lipgloss.Color(theme.Amber))
				text = "✱ "
			default:
				bg := dark
				if (x+y)%2 == 1 {
					bg = light
				}
				st = st.Background(bg)
			}
			if cursor {
				st = st.Background(cursorBg).Foreground(ink)
				if text == "  " {
					text = "▸ "
				}
			}
			sb.WriteString(st.Render(text))
		}
		sb.WriteString(border.Render("│"))
		sb.WriteByte('\n')
	}
	sb.WriteString(border.Render("╰" + strings.Repeat("─", e.d.W*2) + "╯"))
	return sb.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
