// Package snake is the classic single-player snake: one snake, one apple,
// a walled arena.
package snake

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

// Arena size in cells. Each cell is drawn two characters wide so it looks
// roughly square in a terminal.
const (
	Width  = 30
	Height = 18
)

const (
	startLen   = 4
	applePts   = 10
	slowStep   = 140 * time.Millisecond
	fastStep   = 60 * time.Millisecond
	speedUp    = 3 * time.Millisecond // per apple eaten
	growth     = 2                    // segments gained per apple
	queueLimit = 3                    // buffered turns
)

// Game returns the solo snake game.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "snake-classic",
			Name:        "Snake",
			Icon:        "◖",
			Tagline:     "One snake, one apple, four walls.",
			Description: "The original. Eat apples to grow longer and faster, and don't bite the walls or yourself. How long can you get?",
			Kind:        games.Realtime,
			Players:     "1",
			Controls:    []string{"←↑→↓", "turn", "wasd", "turn"},
			Accent:      []string{theme.Lime, theme.Mint, theme.Cyan},
			Art: []string{
				"┌──────────────────────────┐",
				"│   ████████████▶     ◖◗   │",
				"│   █                      │",
				"│   ██████                 │",
				"└──────────────────────────┘",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 20 * time.Millisecond,
	})
}

type point struct{ x, y int }

func (p point) add(q point) point { return point{p.x + q.x, p.y + q.y} }

var (
	up    = point{0, -1}
	down  = point{0, 1}
	left  = point{-1, 0}
	right = point{1, 0}
)

// Engine is a game of snake.
type Engine struct {
	th  *theme.Theme
	rng *rand.Rand

	body  []point // head first
	dir   point
	queue []point
	apple point
	grow  int
	acc   time.Duration

	eaten int
	score int
	state solo.State
}

// New starts a game.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, dir: right}
	cx, cy := Width/2, Height/2
	for i := 0; i < startLen; i++ {
		e.body = append(e.body, point{cx - i, cy})
	}
	e.placeApple()
	return e
}

func (e *Engine) step() time.Duration {
	return max(fastStep, slowStep-time.Duration(e.eaten)*speedUp)
}

func (e *Engine) occupied(p point) bool {
	for _, b := range e.body {
		if b == p {
			return true
		}
	}
	return false
}

func (e *Engine) placeApple() {
	free := Width*Height - len(e.body)
	if free <= 0 {
		e.state = solo.Won
		return
	}
	n := e.rng.IntN(free)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			p := point{x, y}
			if e.occupied(p) {
				continue
			}
			if n == 0 {
				e.apple = p
				return
			}
			n--
		}
	}
}

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	e.acc += dt
	for e.state == solo.Playing && e.acc >= e.step() {
		e.acc -= e.step()
		e.move()
	}
}

func (e *Engine) move() {
	if len(e.queue) > 0 {
		e.dir, e.queue = e.queue[0], e.queue[1:]
	}
	head := e.body[0].add(e.dir)
	if head.x < 0 || head.y < 0 || head.x >= Width || head.y >= Height {
		e.state = solo.Lost
		return
	}
	// The tail moves out of the way this step unless we're growing.
	body := e.body
	if e.grow == 0 {
		body = body[:len(body)-1]
	}
	for _, b := range body {
		if b == head {
			e.state = solo.Lost
			return
		}
	}
	e.body = append([]point{head}, body...)
	if e.grow > 0 {
		e.grow--
	}
	if head == e.apple {
		e.eaten++
		e.score += applePts + e.eaten // later apples are worth a bit more
		e.grow += growth
		e.placeApple()
	}
}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	var d point
	switch k {
	case "up", "w", "k":
		d = up
	case "down", "s", "j":
		d = down
	case "left", "a", "h":
		d = left
	case "right", "d", "l":
		d = right
	default:
		return
	}
	last := e.dir
	if n := len(e.queue); n > 0 {
		last = e.queue[n-1]
	}
	if d == last || d == (point{-last.x, -last.y}) || len(e.queue) >= queueLimit {
		return
	}
	e.queue = append(e.queue, d)
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.score }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{{Label: "LENGTH", Value: strconv.Itoa(len(e.body))}}
}

// View implements solo.Engine.
func (e *Engine) View() string {
	t := e.th
	border := t.Fg(theme.Violet)
	index := make(map[point]int, len(e.body))
	for i, b := range e.body {
		index[b] = i
	}

	var sb strings.Builder
	sb.WriteString(border.Render("╭" + strings.Repeat("─", Width*2) + "╮"))
	sb.WriteByte('\n')
	for y := 0; y < Height; y++ {
		sb.WriteString(border.Render("│"))
		for x := 0; x < Width; x++ {
			p := point{x, y}
			switch i, ok := index[p]; {
			case ok && i == 0:
				c := theme.Lime
				if e.state == solo.Lost {
					c = theme.Coral
				}
				sb.WriteString(t.Fg(c).Bold(true).Render("██"))
			case ok:
				pos := float64(i) / float64(max(len(e.body), 8))
				sb.WriteString(t.Fg(theme.Blend([]string{theme.Lime, theme.Mint, theme.Cyan, theme.Sky}, pos*0.75)).Render("██"))
			case p == e.apple:
				sb.WriteString(t.Fg(theme.Coral).Bold(true).Render("◖◗"))
			case x%3 == 1 && y%2 == 1:
				sb.WriteString(t.Faded.Render("· "))
			default:
				sb.WriteString("  ")
			}
		}
		sb.WriteString(border.Render("│"))
		sb.WriteByte('\n')
	}
	sb.WriteString(border.Render("╰" + strings.Repeat("─", Width*2) + "╯"))
	return sb.String()
}
