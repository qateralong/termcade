package muncher

import (
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), rand.New(rand.NewPCG(seed, seed)))
}

func TestMazeIsWellFormed(t *testing.T) {
	for y, row := range layout {
		if len(row) != MazeW {
			t.Fatalf("row %d is %d wide", y, len(row))
		}
	}
	if strings.Count(mazeString(), "P") != 1 {
		t.Fatal("need exactly one start")
	}
	m := newMaze()
	// Every dot must be reachable from the start.
	seen := map[point]bool{pacStart: true}
	queue := []point{pacStart}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range ghostOrder {
			n := step(p, d)
			if m.open(n) && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	for y := range m.dots {
		for x, d := range m.dots[y] {
			if d != 0 && !seen[point{x, y}] {
				t.Fatalf("dot at (%d,%d) is unreachable", x, y)
			}
		}
	}
	if m.total < 150 {
		t.Fatalf("only %d dots", m.total)
	}
}

// play runs the game for d, starting past the READY pause.
func play(e *Engine, d time.Duration) {
	for step := 10 * time.Millisecond; d > 0; d -= step {
		e.Update(step)
	}
}

func TestPacEatsDots(t *testing.T) {
	e := newTest(1)
	e.ghosts = nil // no interference
	play(e, readyTime+time.Second)
	if e.score == 0 || e.eatenDots == 0 {
		t.Fatalf("score %d after a second of running left", e.score)
	}
}

func TestTunnelWraps(t *testing.T) {
	e := newTest(2)
	e.ghosts = nil
	e.phase = phasePlay
	e.pac.pos, e.pac.dir, e.want = point{0, 10}, dirLeft, dirLeft
	e.movePac()
	if e.pac.pos != (point{MazeW - 1, 10}) {
		t.Fatalf("pac at %v after the tunnel", e.pac.pos)
	}
}

func TestPelletFrightensAndGhostIsEaten(t *testing.T) {
	e := newTest(3)
	e.phase = phasePlay
	g := e.ghosts[0]
	e.pac.pos = point{1, 4}
	e.mz.dots[3][1] = 2 // pellet right above
	e.pac.dir, e.want = dirUp, dirUp
	e.movePac()
	if e.fright == 0 || !g.frightened {
		t.Fatal("pellet didn't frighten Chaser")
	}
	before := e.score
	g.pos = e.pac.pos
	e.collide()
	if g.mode != ghostEaten || e.score != before+200 {
		t.Fatalf("mode=%v score+%d", g.mode, e.score-before)
	}
}

func TestDeathCostsALife(t *testing.T) {
	e := newTest(4)
	e.phase = phasePlay
	g := e.ghosts[0]
	g.pos = e.pac.pos
	e.collide()
	if e.phase != phaseDying {
		t.Fatal("touching a ghost should kill")
	}
	play(e, dyingTime+10*time.Millisecond)
	if e.lives != startLives-1 || e.phase != phaseReady {
		t.Fatalf("lives=%d phase=%v", e.lives, e.phase)
	}
	e.lives = 1
	e.phase, e.phaseTime = phaseDying, 0
	play(e, dyingTime+10*time.Millisecond)
	if e.state != solo.Lost {
		t.Fatal("last life lost should end the game")
	}
}

func TestLevelClears(t *testing.T) {
	e := newTest(5)
	e.phase = phasePlay
	for y := range e.mz.dots {
		for x := range e.mz.dots[y] {
			e.mz.dots[y][x] = 0
		}
	}
	e.mz.dots[pacStart.y][pacStart.x-1] = 1
	e.eatenDots = e.mz.total - 1
	e.pac.dir, e.want = dirLeft, dirLeft
	e.movePac()
	if e.phase != phaseCleared {
		t.Fatal("eating the last dot should clear the level")
	}
	play(e, clearedTime+10*time.Millisecond)
	if e.level != 2 || e.eatenDots != 0 {
		t.Fatalf("level=%d eaten=%d", e.level, e.eatenDots)
	}
}

func TestGhostsLeaveHouseAndSurvive(t *testing.T) {
	// A long simulated game must never crash or push a ghost into a wall.
	e := newTest(6)
	e.lives = 1000
	most := 0 // deaths send ghosts home, so track the most ever out at once
	for i := 0; i < 20000 && e.state == solo.Playing; i++ {
		if i%50 == 0 {
			e.Key([]string{"up", "down", "left", "right"}[e.rng.IntN(4)])
		}
		e.Update(20 * time.Millisecond)
		for _, g := range e.ghosts {
			if e.mz.at(g.pos) == tileWall {
				t.Fatalf("%s is inside a wall at %v", g.name, g.pos)
			}
		}
		if e.mz.at(e.pac.pos) != tileOpen {
			t.Fatalf("pac is off the path at %v", e.pac.pos)
		}
		out := 0
		for _, g := range e.ghosts {
			if g.mode == ghostActive {
				out++
			}
		}
		most = max(most, out)
	}
	if most < 4 {
		t.Fatalf("at most %d ghosts were ever out roaming", most)
	}
}

func TestViewSize(t *testing.T) {
	v := newTest(7).View()
	if w, h := lipgloss.Width(v), lipgloss.Height(v); w != MazeW*2 || h != MazeH {
		t.Fatalf("view is %dx%d", w, h)
	}
}
