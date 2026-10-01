package bomber

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
		out[i] = multi.SeatInfo{Name: "b" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func newTest(n int, bots bool, seed uint64) *World {
	return New(seatsOf(n, bots), rand.New(rand.NewPCG(seed, seed)))
}

func TestSpawnsAreSafe(t *testing.T) {
	for seed := uint64(0); seed < 20; seed++ {
		w := newTest(seats, false, seed)
		for _, p := range w.players {
			free := 0
			for _, d := range dirs {
				if w.walkable(p.pos.add(d)) {
					free++
				}
			}
			if !w.walkable(p.pos) && w.grid[p.pos.y][p.pos.x] != floor || free < 2 {
				t.Fatalf("seed %d: spawn %v is boxed in", seed, p.pos)
			}
		}
	}
}

func TestBlastStopsAtCratesAndWalls(t *testing.T) {
	w := newTest(2, false, 1)
	w.grid = [Rows][Cols]cell{}
	w.grid[3][5] = crate
	w.grid[3][3] = wall
	tiles := w.blast(pt{4, 3}, 3)
	has := map[pt]bool{}
	for _, p := range tiles {
		has[p] = true
	}
	if !has[pt{5, 3}] || has[pt{6, 3}] {
		t.Fatal("flames should reach the crate and stop there")
	}
	if has[pt{3, 3}] {
		t.Fatal("flames went into a wall")
	}
	if !has[pt{4, 6}] || has[pt{4, 7}] {
		t.Fatal("range should be 3 tiles")
	}
}

func TestBombKillsAndChains(t *testing.T) {
	w := newTest(2, false, 2)
	a := w.players[0]
	a.pos, w.players[1].pos = pt{1, 1}, pt{3, 1}
	w.grid[1][2], w.grid[1][3] = floor, floor
	w.bombs = []*bomb{
		{pos: pt{1, 3}, owner: 0, at: time.Hour, power: 2}, // set off by the chain
		{pos: pt{1, 2}, owner: 0, at: 10 * time.Millisecond, power: 2},
	}
	w.grid[2][1], w.grid[3][1] = floor, floor
	w.Step(20 * time.Millisecond)
	if len(w.bombs) != 0 {
		t.Fatal("the second bomb should have gone off in a chain")
	}
	if a.alive {
		t.Fatal("a stood in the blast")
	}
}

func TestCantWalkThroughBombs(t *testing.T) {
	w := newTest(2, false, 3)
	p := w.players[0]
	p.pos = pt{1, 1}
	w.grid[1][2] = floor
	w.bombs = []*bomb{{pos: pt{2, 1}, at: time.Hour, power: 1}}
	w.Input(0, "right")
	if p.pos != (pt{1, 1}) {
		t.Fatal("walked onto a bomb")
	}
	// But you can always walk off your own.
	w.bombs = []*bomb{{pos: pt{1, 1}, at: time.Hour, power: 1}}
	w.Input(0, "right")
	if p.pos != (pt{2, 1}) {
		t.Fatal("couldn't step off a bomb")
	}
}

func TestBotsSurviveTheirOwnBombs(t *testing.T) {
	suicides, games := 0, 0
	var length time.Duration
	for seed := uint64(0); seed < 10; seed++ {
		w := newTest(seats, true, seed)
		for !w.Over() {
			w.Step(50 * time.Millisecond)
		}
		games++
		length += w.clock
		if w.aliveCount() == 0 {
			suicides++
		}
		if st := w.Standings(); len(st) != seats {
			t.Fatal("bad standings")
		}
	}
	t.Logf("%d of %d games ended with everyone dead; average length %v", suicides, games, (length / time.Duration(games)).Round(time.Second))
	if suicides > games/2 {
		t.Errorf("bots blow themselves up too often")
	}
}

func TestViewSize(t *testing.T) {
	w := newTest(seats, true, 5)
	w.Step(time.Second)
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}
