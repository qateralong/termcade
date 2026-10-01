package alien

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
		out[i] = multi.SeatInfo{Name: "s" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func TestBeamHitsOnlyWhereAimed(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(1, 1)))
	w.nextAtk = time.Hour
	a, b := w.saucers[0], w.saucers[1]
	a.angle, b.angle = 0, math.Pi
	a.dir, b.dir = 1, 1
	// A beam aimed where a will be once it fires.
	w.hazards = []*hazard{{kind: beam, start: 0, fire: 100 * time.Millisecond, end: 300 * time.Millisecond, from: 0.2, to: 0.2, half: 0.16}}
	for i := 0; i < 10; i++ {
		w.Step(30 * time.Millisecond)
	}
	if a.alive || !b.alive {
		t.Fatalf("a.alive=%v b.alive=%v", a.alive, b.alive)
	}
	if !w.Over() || w.Standings()[0].Seat != 1 {
		t.Fatal("the survivor should win")
	}
}

func TestSpaceReverses(t *testing.T) {
	w := New(seatsOf(1, false), rand.New(rand.NewPCG(2, 2)))
	d := w.saucers[0].dir
	w.Input(0, " ")
	if w.saucers[0].dir != -d {
		t.Fatal("space didn't reverse the orbit")
	}
}

func TestSaucersBounce(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(3, 3)))
	w.nextAtk = time.Hour
	a, b := w.saucers[0], w.saucers[1]
	a.angle, a.dir = 0, 1
	b.angle, b.dir = 0.5, -1
	for i := 0; i < 10; i++ {
		w.Step(30 * time.Millisecond)
	}
	if a.dir != -1 || b.dir != 1 {
		t.Fatalf("saucers didn't bounce: a.dir=%v b.dir=%v", a.dir, b.dir)
	}
}

func TestBotsOutlastIdlers(t *testing.T) {
	// Over many matches, bots that dodge should outlive saucers that never
	// react, and matches should last a reasonable time.
	botWins, total := 0, 0
	var length time.Duration
	for seed := uint64(0); seed < 12; seed++ {
		seats := seatsOf(seats, true)
		seats[0].Bot, seats[1].Bot = false, false // two idle "humans"
		w := New(seats, rand.New(rand.NewPCG(seed, 9)))
		for !w.Over() {
			w.Step(50 * time.Millisecond)
		}
		total++
		length += w.clock
		if win := w.Standings()[0].Seat; win >= 2 {
			botWins++
		}
	}
	avg := length / time.Duration(total)
	t.Logf("bots won %d of %d, average match %v", botWins, total, avg.Round(time.Second))
	if botWins < total*3/4 {
		t.Errorf("dodging bots only won %d of %d", botWins, total)
	}
	if avg < 15*time.Second {
		t.Errorf("matches are too short (%v)", avg)
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(4, 4)))
	for i := 0; i < 60; i++ {
		w.Step(50 * time.Millisecond)
	}
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}
