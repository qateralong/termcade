package snake

import (
	"io"
	"math/rand/v2"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), rand.New(rand.NewPCG(seed, seed)))
}

func TestEatAndGrow(t *testing.T) {
	e := newTest(1)
	head := e.body[0]
	e.apple = point{head.x + 1, head.y}
	e.move()
	if e.score == 0 || e.eaten != 1 {
		t.Fatalf("apple not eaten: score=%d", e.score)
	}
	before := len(e.body)
	e.move()
	e.move()
	if len(e.body) != before+growth {
		t.Fatalf("length %d, want %d", len(e.body), before+growth)
	}
	if e.occupied(e.apple) {
		t.Fatal("apple spawned on the snake")
	}
}

func TestWallKills(t *testing.T) {
	e := newTest(2)
	for i := 0; i < Width && e.state == solo.Playing; i++ {
		e.apple = point{-5, -5} // keep it out of the way
		e.move()
	}
	if e.state != solo.Lost {
		t.Fatal("snake survived hitting the wall")
	}
}

func TestSelfCollision(t *testing.T) {
	e := newTest(3)
	e.apple = point{-5, -5}
	e.body = []point{{5, 5}, {4, 5}, {4, 6}, {5, 6}, {6, 6}, {6, 5}}
	e.dir = down // turning into its own body
	e.move()
	if e.state != solo.Lost {
		t.Fatal("snake survived biting itself")
	}
}

func TestChasingTailIsSafe(t *testing.T) {
	e := newTest(4)
	e.apple = point{-5, -5}
	// A 2x2 loop: the head moves into the cell the tail is leaving.
	e.body = []point{{5, 5}, {5, 6}, {6, 6}, {6, 5}}
	e.dir = right
	e.move()
	if e.state != solo.Playing {
		t.Fatal("moving into the vacating tail should be allowed")
	}
}

func TestNoReverse(t *testing.T) {
	e := newTest(5)
	e.Key("left") // opposite of the initial direction
	if len(e.queue) != 0 {
		t.Fatal("reverse turn was queued")
	}
	e.Key("up")
	e.Key("left") // fine after turning up
	if len(e.queue) != 2 {
		t.Fatalf("queue = %v", e.queue)
	}
}

func TestViewSize(t *testing.T) {
	v := newTest(6).View()
	if w, h := lipgloss.Width(v), lipgloss.Height(v); w != Width*2+2 || h != Height+2 {
		t.Fatalf("view is %dx%d", w, h)
	}
}
