package snakearena

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
		out[i] = multi.SeatInfo{Name: "p" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func TestBotsPlayAFullRound(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(1, 1)))
	for !w.Over() {
		w.Step(50 * time.Millisecond)
		for _, s := range w.snakes {
			if !s.alive {
				continue
			}
			for _, b := range s.body {
				if !inside(b) {
					t.Fatalf("%s is outside the arena at %v", s.info.Name, b)
				}
			}
		}
	}
	total := 0
	for _, s := range w.snakes {
		total += s.score
	}
	if total < 20 {
		t.Fatalf("bots only scored %d points in a whole round", total)
	}
	st := w.Standings()
	for i := 1; i < len(st); i++ {
		if w.snakes[st[i-1].Seat].score < w.snakes[st[i].Seat].score {
			t.Fatal("standings are not sorted")
		}
	}
}

func TestHeadOnKillsBoth(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(2, 2)))
	a, b := w.snakes[0], w.snakes[1]
	a.body = []pt{{10, 10}, {9, 10}, {8, 10}}
	a.dir = pt{1, 0}
	b.body = []pt{{12, 10}, {13, 10}, {14, 10}}
	b.dir = pt{-1, 0}
	w.dots = map[pt]bool{}
	w.move()
	if a.alive || b.alive {
		t.Fatal("head-on collision should kill both")
	}
	w.clock = respawnTime
	w.Step(time.Millisecond) // respawns without moving yet
	if !a.alive || !b.alive {
		t.Fatal("snakes should respawn")
	}
}

func TestCrashIntoBodyGivesBonus(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(3, 3)))
	a, b := w.snakes[0], w.snakes[1]
	a.body = []pt{{10, 9}, {10, 8}, {10, 7}}
	a.dir = pt{0, 1} // moving down into b's body
	b.body = []pt{{12, 10}, {11, 10}, {10, 10}, {9, 10}}
	b.dir = pt{1, 0}
	w.dots = map[pt]bool{}
	w.move()
	if a.alive || !b.alive || b.score != killBonus {
		t.Fatalf("a.alive=%v b.alive=%v b.score=%d", a.alive, b.alive, b.score)
	}
}

func TestViewFitsTheArena(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(4, 4)))
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}
