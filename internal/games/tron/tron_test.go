package tron

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
		out[i] = multi.SeatInfo{Name: "r" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func TestCrashIntoTrailCreditsTheOwner(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(1, 1)))
	a, b := w.riders[0], w.riders[1]
	a.pos, a.dir = pt{10, 10}, pt{1, 0}
	b.pos, b.dir = pt{30, 30}, pt{0, -1}
	w.trail[10][11] = 2 // b's wall right in front of a
	w.move(func(*rider) bool { return true })
	if a.alive || b.kills != 1 {
		t.Fatalf("a.alive=%v b.kills=%d", a.alive, b.kills)
	}
	if !w.Over() || w.Standings()[0].Seat != 1 {
		t.Fatal("the survivor should win")
	}
}

func TestHeadOn(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(2, 2)))
	a, b := w.riders[0], w.riders[1]
	a.pos, a.dir = pt{10, 10}, pt{1, 0}
	b.pos, b.dir = pt{12, 10}, pt{-1, 0}
	w.move(func(*rider) bool { return true })
	if a.alive || b.alive {
		t.Fatal("head-on should take out both")
	}
}

func TestBoostMovesTwice(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(3, 3)))
	r := w.riders[0]
	start := r.pos
	w.Input(0, " ")
	w.Step(w.interval())
	if d := r.pos.x - start.x + r.pos.y - start.y; d != 2 && d != -2 {
		t.Fatalf("boosted rider moved %v -> %v", start, r.pos)
	}
	w.Input(0, " ") // still cooling down
	if r.boostAt != 0 {
		t.Fatal("boost should be on cooldown")
	}
}

func TestBotsLastAWhile(t *testing.T) {
	var total time.Duration
	for seed := uint64(0); seed < 6; seed++ {
		w := New(seatsOf(seats, true), rand.New(rand.NewPCG(seed, 4)))
		for !w.Over() {
			w.Step(50 * time.Millisecond)
		}
		total += w.clock
		if st := w.Standings(); len(st) != seats {
			t.Fatal("bad standings")
		}
	}
	avg := total / 6
	t.Logf("bot rounds last %v on average", avg.Round(time.Second))
	if avg < 10*time.Second {
		t.Fatalf("rounds are too short (%v)", avg)
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(5, 5)))
	w.Step(time.Second)
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}
