package invaders

import (
	"io"
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

func TestShootingAnInvader(t *testing.T) {
	e := newTest(1)
	e.bombCD = time.Hour
	e.shields = nil // nothing in the way
	// Line up under the bottom-left invader.
	x, _ := e.alienPos(rows-1, 0)
	e.player = x + 1 - playerW/2
	e.Key(" ")
	for i := 0; i < 80 && e.count == rows*cols; i++ {
		e.Update(25 * time.Millisecond)
	}
	if e.count != rows*cols-1 || e.score != rowPoints[rows-1] {
		t.Fatalf("count %d score %d", e.count, e.score)
	}
}

func TestFleetSpeedsUp(t *testing.T) {
	e := newTest(2)
	full := e.marchInterval()
	for r := 0; r < rows; r++ {
		for c := 0; c < cols-1; c++ {
			e.alive[r][c] = false
			e.count--
		}
	}
	if e.marchInterval() >= full/3 {
		t.Fatalf("interval %v vs %v at full strength", e.marchInterval(), full)
	}
}

func TestBombHitsPlayer(t *testing.T) {
	e := newTest(3)
	e.bombCD = time.Hour
	e.bombs = []*shot{{x: e.player + 2, y: playerY - 2}}
	for i := 0; i < 20; i++ {
		e.Update(25 * time.Millisecond)
	}
	if e.lives != startLives-1 {
		t.Fatalf("lives %d", e.lives)
	}
}

func TestIdlePlayerEventuallyLoses(t *testing.T) {
	e := newTest(4)
	for i := 0; i < 200000 && e.state == solo.Playing; i++ {
		e.Update(25 * time.Millisecond)
	}
	if e.state != solo.Lost {
		t.Fatal("a player who never moves should lose")
	}
}

func TestShootingEverythingStartsAWave(t *testing.T) {
	e := newTest(5)
	for r := range e.alive {
		for c := range e.alive[r] {
			e.alive[r][c] = false
		}
	}
	e.count = 0
	e.Update(25 * time.Millisecond)
	if e.wave != 2 || e.count != rows*cols {
		t.Fatalf("wave %d count %d", e.wave, e.count)
	}
}

func TestViewSize(t *testing.T) {
	v := newTest(6).View()
	if w, h := lipgloss.Width(v), lipgloss.Height(v); w != W+2 || h != H/2+2 {
		t.Fatalf("view is %dx%d", w, h)
	}
}
