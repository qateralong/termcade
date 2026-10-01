package battleship

import (
	"io"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/multi"
	"termcade/internal/ui/theme"
)

func seatsOf(bots bool) []multi.SeatInfo {
	return []multi.SeatInfo{
		{Name: "anna", Color: theme.Pink, Bot: bots},
		{Name: "boris", Color: theme.Cyan, Bot: bots},
	}
}

func TestRandomBoardsFollowTheRules(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	for i := 0; i < 200; i++ {
		b := randomBoard(rng)
		if len(b.ships) != len(fleet) {
			t.Fatalf("placed %d ships", len(b.ships))
		}
		// No two ships touch, even diagonally.
		owner := map[cell]int{}
		for si, s := range b.ships {
			for _, c := range s {
				owner[c] = si
			}
		}
		for c, si := range owner {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if o, ok := owner[cell{c.x + dx, c.y + dy}]; ok && o != si {
						t.Fatalf("ships %d and %d touch at %v", si, o, c)
					}
				}
			}
		}
		if b.afloat() != 20 {
			t.Fatalf("%d ship cells", b.afloat())
		}
	}
}

func TestSinkingMarksSurroundings(t *testing.T) {
	b := &board{}
	b.place(4, 4, 2, true) // cells (4,4) and (5,4)
	if r := b.shoot(4, 4); r != resHit {
		t.Fatalf("first shot = %v", r)
	}
	if r := b.shoot(4, 4); r != resInvalid {
		t.Fatal("shooting the same cell twice should be invalid")
	}
	if r := b.shoot(5, 4); r != resSunk {
		t.Fatalf("second shot = %v", r)
	}
	for _, c := range []cell{{3, 3}, {6, 5}, {3, 4}, {6, 4}} {
		if b.grid[c.y][c.x] != miss {
			t.Fatalf("cell %v around the sunk ship not marked", c)
		}
	}
	if b.shipsLeft() != 0 {
		t.Fatal("ship should be sunk")
	}
}

func TestHitGivesAnotherShot(t *testing.T) {
	w := New(seatsOf(false), rand.New(rand.NewPCG(2, 2)))
	w.phase, w.turn = battle, 0
	s := w.p[1].board.ships[0]
	w.fire(0, s[0].x, s[0].y)
	if w.turn != 0 {
		t.Fatal("a hit should keep the turn")
	}
	// Find water and miss.
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			if w.p[1].board.grid[y][x] == water {
				w.fire(0, x, y)
				if w.turn != 1 {
					t.Fatal("a miss should pass the turn")
				}
				return
			}
		}
	}
}

func TestBotsFinishAMatch(t *testing.T) {
	var shots []int
	for seed := uint64(0); seed < 10; seed++ {
		w := New(seatsOf(true), rand.New(rand.NewPCG(seed, 3)))
		for !w.Over() && w.clock < time.Hour {
			w.Step(100 * time.Millisecond)
		}
		if !w.Over() {
			t.Fatal("bots never finished")
		}
		shots = append(shots, w.p[w.winner].shots)
		if w.p[1-w.winner].board.shipsLeft() != 0 {
			t.Fatal("winner's opponent still has ships")
		}
	}
	total := 0
	for _, s := range shots {
		total += s
	}
	avg := total / len(shots)
	t.Logf("winning bots needed %d shots on average", avg)
	// Random shooting needs ~95 shots; a decent hunter far fewer.
	if avg > 70 {
		t.Errorf("bot is a poor shot: %d shots on average", avg)
	}
}

func TestTimeoutShootsForYou(t *testing.T) {
	w := New(seatsOf(false), rand.New(rand.NewPCG(4, 4)))
	w.Input(0, "enter")
	w.Input(1, "enter")
	w.Step(10 * time.Millisecond)
	if w.phase != battle {
		t.Fatal("both ready should start the battle")
	}
	shooter := w.turn
	w.Step(turnTime + time.Millisecond)
	if w.p[shooter].shots != 1 {
		t.Fatal("timeout didn't fire a shot")
	}
}

func TestViewSize(t *testing.T) {
	w := New(seatsOf(true), rand.New(rand.NewPCG(5, 5)))
	th := theme.New(lipgloss.NewRenderer(io.Discard))
	for _, ph := range []phase{placing, battle, over} {
		w.phase = ph
		v := w.View(0, th)
		if lw, lh := lipgloss.Width(v), lipgloss.Height(v); lw != ViewW || lh != ViewH {
			t.Fatalf("phase %d: view is %dx%d", ph, lw, lh)
		}
	}
}
