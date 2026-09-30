package racing

import (
	"io"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/multi"
	"termcade/internal/ui/theme"
)

func seatsOf(n int, bots bool) []multi.SeatInfo {
	out := make([]multi.SeatInfo, n)
	for i := range out {
		out[i] = multi.SeatInfo{Name: "r" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func TestTracksFitAndStartOnRoad(t *testing.T) {
	for n, spec := range tracks {
		for _, p := range spec.pts {
			if p.x < spec.width/2+1 || p.y < spec.width/2+1 || p.x > W-spec.width/2-1 || p.y > H-spec.width/2-1 {
				t.Errorf("%s: waypoint %v too close to the edge", spec.name, p)
			}
		}
		w := New(seatsOf(seats, true), rand.New(rand.NewPCG(1, 1)), n)
		for _, c := range w.cars {
			if w.tr.surface(c.pos) == 0 {
				t.Errorf("%s: %s starts on the grass at %v", spec.name, c.info.Name, c.pos)
			}
		}
	}
}

func TestBotsFinishEveryTrack(t *testing.T) {
	for n, spec := range tracks {
		w := New(seatsOf(seats, true), rand.New(rand.NewPCG(uint64(n), 2)), n)
		for !w.Over() {
			w.Step(50 * time.Millisecond)
		}
		if len(w.order) == 0 {
			t.Errorf("%s: no bot finished within the time limit", spec.name)
			continue
		}
		t.Logf("%s: winner in %v, %d of %d finished", spec.name,
			w.cars[w.order[0]].doneAt.Round(100*time.Millisecond), len(w.order), seats)
		if st := w.Standings(); len(st) != seats {
			t.Fatalf("standings has %d rows", len(st))
		}
	}
}

func TestCuttingAcrossGrassIsSlow(t *testing.T) {
	w := New(seatsOf(1, false), rand.New(rand.NewPCG(3, 3)), 0)
	c := w.cars[0]
	c.pos = vec{30, 18} // the infield
	c.speed = topSpeed
	c.gas = time.Hour
	for i := 0; i < 20; i++ {
		w.Step(50 * time.Millisecond)
	}
	if c.speed > grassSpeed+0.01 {
		t.Fatalf("speed on grass is %.1f", c.speed)
	}
}

func TestLapCounting(t *testing.T) {
	w := New(seatsOf(1, false), rand.New(rand.NewPCG(4, 4)), 0)
	c := w.cars[0]
	// Teleport around the track in small steps.
	for s := 0.0; s < w.tr.length*laps+5; s += 1 {
		c.pos = w.tr.pointAt(c.along + 1)
		c.speed = 0
		w.move(c, 0)
		w.Step(0)
	}
	if !c.done || w.lap(c) != laps {
		t.Fatalf("done=%v lap=%d dist=%.0f of %.0f", c.done, w.lap(c), c.dist, w.tr.length*laps)
	}
}

func TestSteeringWraps(t *testing.T) {
	w := New(seatsOf(1, false), rand.New(rand.NewPCG(5, 5)), 1)
	c := w.cars[0]
	h := c.heading
	for i := 0; i < turnSteps; i++ {
		w.Input(0, "left")
	}
	if c.heading != h {
		t.Fatal("a full turn should come back to the same heading")
	}
	if math.IsNaN(c.angle()) {
		t.Fatal("bad angle")
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(6, 6)), 2)
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}
