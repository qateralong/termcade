package breakout

import (
	"io"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), rand.New(rand.NewPCG(seed, seed)))
}

// autopilot keeps the paddle under the ball.
func autopilot(e *Engine) {
	center := e.paddle + paddleW/2
	switch {
	case e.bx < center-1:
		e.Key("left")
	case e.bx > center+1:
		e.Key("right")
	}
}

func TestAutopilotClearsALevel(t *testing.T) {
	e := newTest(1)
	e.Key(" ")
	for i := 0; i < 200000 && e.level == 1 && e.state == solo.Playing; i++ {
		autopilot(e)
		e.Update(10 * time.Millisecond)
		if e.stuck {
			e.Key(" ")
		}
		if e.bx < -1 || e.bx > W+1 || e.by < -1 {
			t.Fatalf("ball escaped to (%.1f, %.1f)", e.bx, e.by)
		}
	}
	if e.level != 2 {
		t.Fatalf("autopilot didn't clear level 1 (bricks left %d, lives %d)", e.left, e.lives)
	}
	if e.score != 12*(70+60+50+30+20+10) {
		t.Fatalf("score %d", e.score)
	}
}

func TestMissingCostsALife(t *testing.T) {
	e := newTest(2)
	e.Key(" ")
	e.paddle = 0
	e.bx, e.by, e.vx, e.vy = W-5, H-5, 0, 30
	e.Update(time.Second)
	if e.lives != startLives-1 || !e.stuck {
		t.Fatalf("lives %d stuck %v", e.lives, e.stuck)
	}
}

func TestPaddleAngle(t *testing.T) {
	e := newTest(3)
	e.Key(" ")
	e.paddle = 20
	// Hit the right edge of the paddle: the ball should head right.
	e.bx, e.by, e.vx, e.vy = e.paddle+paddleW-0.5, paddleY-0.5, 0, 20
	e.Update(50 * time.Millisecond)
	if e.vx <= 0 || e.vy >= 0 {
		t.Fatalf("vx %.1f vy %.1f", e.vx, e.vy)
	}
	if sp := math.Hypot(e.vx, e.vy); math.Abs(sp-e.speed()) > 0.01 {
		t.Fatalf("speed %.2f", sp)
	}
}

func TestViewSize(t *testing.T) {
	v := newTest(4).View()
	if w, h := lipgloss.Width(v), lipgloss.Height(v); w != W+2 || h != H/2+2 {
		t.Fatalf("view is %dx%d", w, h)
	}
}
