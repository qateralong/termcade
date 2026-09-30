package chickenrun

import (
	"io"
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
		out[i] = multi.SeatInfo{Name: "c" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func TestLevelsAreDeterministic(t *testing.T) {
	for _, spec := range levels {
		a, b := buildLevel(spec), buildLevel(spec)
		for r := range a.solid {
			for c := range a.solid[r] {
				if a.solid[r][c] != b.solid[r][c] {
					t.Fatalf("%s differs between builds", spec.name)
				}
			}
		}
		// The start is safe: flat floor and ceiling.
		for c := 0; c < startRun; c++ {
			if !a.at(c, levelRows-1) || !a.at(c, 0) || a.at(c, levelRows/2) {
				t.Fatalf("%s: start area isn't clear at column %d", spec.name, c)
			}
		}
	}
}

func TestFlipOnlyWhenGrounded(t *testing.T) {
	w := New(seatsOf(1, false), rand.New(rand.NewPCG(1, 1)), 0)
	c := w.chickens[0]
	w.Step(100 * time.Millisecond) // settle on the floor
	if !w.grounded(c) {
		t.Fatal("chicken should start on the floor")
	}
	w.Input(0, " ")
	if c.grav != -1 {
		t.Fatal("flip didn't change gravity")
	}
	w.Step(50 * time.Millisecond) // now in the air
	w.Input(0, " ")
	if c.grav != -1 {
		t.Fatal("flipped in mid-air")
	}
	for i := 0; i < 40; i++ {
		w.Step(50 * time.Millisecond)
	}
	if !w.grounded(c) || c.y > float64(4*tile) {
		t.Fatalf("chicken should have landed on the ceiling, y=%.1f", c.y)
	}
}

func TestBotsCanRaceEveryLevel(t *testing.T) {
	for n, spec := range levels {
		best := 0.0
		for seed := uint64(0); seed < 3; seed++ {
			w := New(seatsOf(seats, true), rand.New(rand.NewPCG(seed, uint64(n))), n)
			for !w.Over() && w.clock < 3*time.Minute {
				w.Step(50 * time.Millisecond)
			}
			for _, c := range w.chickens {
				best = max(best, c.x/float64(w.lv.finish*tile))
			}
			if st := w.Standings(); len(st) != seats {
				t.Fatalf("standings has %d rows", len(st))
			}
		}
		t.Logf("%s: best bot got %.0f%% of the way", spec.name, best*100)
		if best < 0.5 {
			t.Errorf("%s: bots never got past halfway (best %.0f%%)", spec.name, best*100)
		}
	}
}

func TestFallingBehindKnocksOut(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(2, 2)), 0)
	w.chickens[1].x = -10 // way behind the camera
	w.camX = 5
	w.Step(10 * time.Millisecond)
	if w.chickens[1].state != out {
		t.Fatal("chicken off the left edge should be out")
	}
	if !w.Over() {
		t.Fatal("race should end with one chicken left")
	}
	if st := w.Standings(); st[0].Seat != 0 {
		t.Fatal("the survivor should win")
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(3, 3)), 2)
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}

func TestDoingNothingLoses(t *testing.T) {
	for n, spec := range levels {
		// One idle "human" racing a bot: the idle chicken must not survive.
		seats := seatsOf(2, false)
		seats[1].Bot = true
		w := New(seats, rand.New(rand.NewPCG(9, uint64(n))), n)
		for !w.Over() && w.clock < 3*time.Minute {
			w.Step(50 * time.Millisecond)
		}
		idle := w.chickens[0]
		if idle.state != out {
			t.Errorf("%s: a chicken that never flips survived (%s)", spec.name, w.progress(idle))
		} else {
			t.Logf("%s: idle chicken %s at %s after %v", spec.name, idle.why, w.progress(idle), idle.doneAt.Round(time.Second))
		}
	}
}
