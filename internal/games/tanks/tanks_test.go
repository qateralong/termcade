package tanks

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
		out[i] = multi.SeatInfo{Name: "t" + string(rune('a'+i)), Color: theme.PlayerColors[i].Hex, Bot: bots}
	}
	return out
}

func TestMapsAreFairAndConnected(t *testing.T) {
	for mi, q := range quadrants {
		for y, row := range q.rows {
			if len(row) != 15 {
				t.Fatalf("%s row %d is %d wide", q.name, y, len(row))
			}
		}
		tiles := buildMap(q.rows)
		// Tanks cover 2×2 tiles; from the first spawn, every other spawn must
		// be reachable when bricks can be shot away.
		ok := func(x, y int) bool {
			if x < 0 || y < 0 || x+1 >= TilesW || y+1 >= TilesH {
				return false
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					if c := tiles[y+dy][x+dx]; c == '@' || c == '~' {
						return false
					}
				}
			}
			return true
		}
		start := spawns[0]
		seen := map[[2]int]bool{start: true}
		queue := [][2]int{start}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
				n := [2]int{p[0] + d[0], p[1] + d[1]}
				if !seen[n] && ok(n[0], n[1]) {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
		for _, s := range spawns {
			if !ok(s[0], s[1]) || !seen[s] {
				t.Errorf("map %d (%s): spawn %v unreachable", mi, q.name, s)
			}
		}
	}
}

func TestShellDestroysBrickAndTank(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(1, 1)), 0)
	a, b := w.tanks[0], w.tanks[1]
	// Clear the field and line the tanks up.
	w.pix = [H][W]byte{}
	a.x, a.y, a.dir = 10, 10, right
	b.x, b.y = 30, 10
	a.since, b.since = -time.Hour, -time.Hour // no spawn shields
	w.pix[11][20] = brick
	w.pix[12][20] = brick

	w.Input(0, " ")
	for i := 0; i < 40; i++ {
		w.Step(20 * time.Millisecond)
	}
	if w.pix[11][20] == brick && w.pix[12][20] == brick {
		t.Fatal("brick survived a direct hit")
	}
	for i := 0; i < 40; i++ {
		w.Input(0, " ")
		w.Step(50 * time.Millisecond)
	}
	if b.lives != startLives-1 || a.kills != 1 {
		t.Fatalf("b.lives=%d a.kills=%d", b.lives, a.kills)
	}
}

func TestShieldBlocksDamage(t *testing.T) {
	w := New(seatsOf(2, false), rand.New(rand.NewPCG(2, 2)), 0)
	a, b := w.tanks[0], w.tanks[1]
	w.pix = [H][W]byte{}
	a.x, a.y, a.dir = 10, 10, right
	b.x, b.y = 20, 10
	a.since = -time.Hour
	w.Input(0, " ")
	for i := 0; i < 20; i++ {
		w.Step(20 * time.Millisecond)
	}
	if b.lives != startLives {
		t.Fatal("shielded tank took damage")
	}
}

func TestBotsFinishABattle(t *testing.T) {
	for m := range quadrants {
		w := New(seatsOf(seats, true), rand.New(rand.NewPCG(uint64(m)+3, 7)), m)
		for !w.Over() {
			w.Step(50 * time.Millisecond)
			for _, tk := range w.tanks {
				if tk.alive && !w.fits(tk, tk.x, tk.y) {
					t.Fatalf("map %d: tank %s is inside something at (%d,%d)", m, tk.info.Name, tk.x, tk.y)
				}
			}
		}
		kills := 0
		for _, tk := range w.tanks {
			kills += tk.kills
		}
		if kills < 3 {
			t.Errorf("map %d: bots only scored %d kills in %v", m, kills, w.clock)
		}
		t.Logf("map %d (%s): over after %v with %d kills", m, quadrants[m].name, w.clock.Round(time.Second), kills)
		if st := w.Standings(); len(st) != seats {
			t.Fatalf("standings has %d rows", len(st))
		}
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(seats, true), rand.New(rand.NewPCG(4, 4)), 1)
	v := w.View(0, theme.New(lipgloss.NewRenderer(io.Discard)))
	if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != multi.ArenaW+2+1+multi.SidebarW || lh != multi.ArenaH/2+2 {
		t.Fatalf("view is %dx%d", lw, lh)
	}
}
