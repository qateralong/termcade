// Package breakout is the brick-breaking classic: keep the ball in play with
// your paddle and clear every brick.
package breakout

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

const (
	W = 60 // pixels
	H = 36

	brickW, brickH = 4, 2
	brickCols      = 12
	brickRows      = 6
	bricksTop      = 4

	paddleW    = 9
	paddleY    = H - 2
	paddleStep = 3.0
	startLives = 3
	baseSpeed  = 24.0
	speedStep  = 3.0
	maxSpeed   = 42.0
)

var rowColors = []string{theme.Coral, "#FF9F43", theme.Amber, theme.Lime, theme.Cyan, theme.Violet}
var rowPoints = []int{70, 60, 50, 30, 20, 10}

// Game returns Breakout.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "breakout",
			Name:        "Breakout",
			Icon:        "▬",
			Tagline:     "One ball, one paddle, a wall of bricks.",
			Description: "Bounce the ball off your paddle to smash every brick. Where the ball hits the paddle decides its angle. Clear the wall to move on to a faster level.",
			Kind:        games.Realtime,
			Players:     "1",
			Controls:    []string{"←→", "move (hold)", "space", "launch"},
			Accent:      []string{theme.Coral, theme.Amber, theme.Cyan},
			Art: []string{
				"▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀",
				"▀▀▀▀ ▀▀▀▀      ▀▀▀▀ ▀▀▀▀",
				"            ●",
				"         ▀▀▀▀▀▀▀",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 25 * time.Millisecond,
	})
}

// Engine is one game of Breakout.
type Engine struct {
	th     *theme.Theme
	rng    *rand.Rand
	bricks [brickRows][brickCols]bool
	left   int
	paddle float64 // left edge
	bx, by float64
	vx, vy float64
	stuck  bool // ball resting on the paddle, waiting for launch
	score  int
	lives  int
	level  int
	state  solo.State
	flash  time.Duration
}

// New starts a game.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, lives: startLives, level: 1}
	e.fill()
	e.reset()
	return e
}

func (e *Engine) fill() {
	for r := range e.bricks {
		for c := range e.bricks[r] {
			e.bricks[r][c] = true
		}
	}
	e.left = brickRows * brickCols
}

// reset puts the ball back on the paddle.
func (e *Engine) reset() {
	e.paddle = (W - paddleW) / 2
	e.stuck = true
	e.placeOnPaddle()
}

func (e *Engine) placeOnPaddle() {
	e.bx, e.by = e.paddle+paddleW/2, paddleY-1
}

func (e *Engine) speed() float64 {
	return math.Min(maxSpeed, baseSpeed+speedStep*float64(e.level-1))
}

func (e *Engine) launch() {
	if !e.stuck {
		return
	}
	e.stuck = false
	angle := (-0.5 + e.rng.Float64()) * 0.9 // up, a little left or right
	e.vx = e.speed() * math.Sin(angle)
	e.vy = -e.speed() * math.Cos(angle)
}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	switch k {
	case "left", "a", "h":
		e.paddle = math.Max(0, e.paddle-paddleStep)
	case "right", "d", "l":
		e.paddle = math.Min(W-paddleW, e.paddle+paddleStep)
	case " ", "up", "w", "enter":
		e.launch()
	}
	if e.stuck {
		e.placeOnPaddle()
	}
}

// brickAt returns the brick covering pixel (x, y).
func brickAt(x, y int) (int, int, bool) {
	if y < bricksTop || y >= bricksTop+brickRows*brickH || x < 0 || x >= W {
		return 0, 0, false
	}
	c := x / (W / brickCols)
	if x%(W/brickCols) >= brickW {
		return 0, 0, false // the gap between bricks
	}
	return (y - bricksTop) / brickH, c, true
}

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	if e.flash > 0 {
		e.flash -= dt
	}
	if e.stuck || e.state != solo.Playing {
		return
	}
	// Move in small steps so the ball never skips through a brick.
	dist := math.Hypot(e.vx, e.vy) * dt.Seconds()
	steps := int(math.Ceil(dist / 0.4))
	for i := 0; i < steps && !e.stuck; i++ {
		e.advance(dt.Seconds() / float64(steps))
	}
}

func (e *Engine) advance(secs float64) {
	px, py := e.bx, e.by
	e.bx += e.vx * secs
	e.by += e.vy * secs

	// Walls.
	if e.bx < 0 {
		e.bx, e.vx = -e.bx, math.Abs(e.vx)
	}
	if e.bx >= W {
		e.bx, e.vx = 2*W-e.bx-0.01, -math.Abs(e.vx)
	}
	if e.by < 0 {
		e.by, e.vy = -e.by, math.Abs(e.vy)
	}

	// Paddle: the bounce angle depends on where the ball lands.
	if e.vy > 0 && py < paddleY && e.by >= paddleY && e.bx >= e.paddle-0.5 && e.bx <= e.paddle+paddleW+0.5 {
		off := (e.bx - (e.paddle + paddleW/2)) / (paddleW / 2) // -1..1
		off = math.Max(-1, math.Min(1, off))
		angle := off * 1.05 // up to ~60°
		sp := e.speed()
		e.vx, e.vy = sp*math.Sin(angle), -sp*math.Cos(angle)
		e.by = paddleY - 0.01
	}

	// Lost the ball.
	if e.by >= H {
		e.lives--
		e.flash = 400 * time.Millisecond
		if e.lives <= 0 {
			e.state = solo.Lost
			return
		}
		e.reset()
		return
	}

	// Bricks.
	if r, c, ok := brickAt(int(e.bx), int(e.by)); ok && e.bricks[r][c] {
		e.bricks[r][c] = false
		e.left--
		e.score += rowPoints[r]
		pr, pc, pok := brickAt(int(px), int(py))
		if pok && pr == r && pc != c {
			e.vx = -e.vx // came in from the side
		} else if int(py) != int(e.by) {
			e.vy = -e.vy
		} else {
			e.vx = -e.vx
		}
		e.bx, e.by = px, py
		if e.left == 0 {
			e.level++
			e.fill()
			e.reset()
		}
	}
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.score }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{
		{Label: "LIVES", Value: strings.Repeat("●", max(e.lives, 0))},
		{Label: "LEVEL", Value: strconv.Itoa(e.level)},
	}
}

// View implements solo.Engine.
func (e *Engine) View() string {
	c := canvas.New(W, H)
	for r := 0; r < brickRows; r++ {
		for col := 0; col < brickCols; col++ {
			if !e.bricks[r][col] {
				continue
			}
			x, y := col*(W/brickCols), bricksTop+r*brickH
			c.Fill(x, y, brickW, 1, rowColors[r])
			c.Fill(x, y+1, brickW, 1, theme.Mix(rowColors[r], "#000000", 0.3))
		}
	}
	pc := "#E8E3F7"
	if e.flash > 0 {
		pc = theme.Coral
	}
	c.Fill(int(math.Round(e.paddle)), paddleY, paddleW, 1, pc)
	c.Set(int(e.bx), int(e.by), "#FFFFFF")
	if e.stuck {
		c.TextCenter(H/2/2+4, " space to launch ", "#1A1325", theme.Amber)
	}
	return e.th.Panel.BorderForeground(lipgloss.Color(theme.Violet)).Render(c.Render(e.th.R.ColorProfile()))
}
