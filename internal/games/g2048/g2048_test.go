package g2048

import (
	"io"
	"math/rand/v2"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), rand.New(rand.NewPCG(seed, seed)))
}

func TestSlideRow(t *testing.T) {
	cases := []struct {
		in   [N]int
		want [N]int
		pts  int
	}{
		{[N]int{2, 2, 0, 0}, [N]int{4, 0, 0, 0}, 4},
		{[N]int{2, 2, 2, 2}, [N]int{4, 4, 0, 0}, 8},
		{[N]int{4, 4, 8, 0}, [N]int{8, 8, 0, 0}, 8}, // merged tiles don't merge again
		{[N]int{0, 2, 0, 2}, [N]int{4, 0, 0, 0}, 4},
		{[N]int{2, 4, 8, 16}, [N]int{2, 4, 8, 16}, 0},
		{[N]int{8, 0, 8, 8}, [N]int{16, 8, 0, 0}, 16},
	}
	for _, c := range cases {
		got, pts := slideRow(c.in)
		if got != c.want || pts != c.pts {
			t.Errorf("slideRow(%v) = %v, %d; want %v, %d", c.in, got, pts, c.want, c.pts)
		}
	}
}

func TestMoveDirections(t *testing.T) {
	e := newTest(1)
	e.board = [N][N]int{{2, 0, 0, 2}}
	e.Key("right")
	if e.board[0][3] != 4 || e.score != 4 {
		t.Fatalf("right: %v score %d", e.board[0], e.score)
	}
	e.board = [N][N]int{}
	e.board[0][1], e.board[3][1] = 8, 8
	e.Key("up")
	if e.board[0][1] != 16 {
		t.Fatalf("up: column = %d", e.board[0][1])
	}
	e.Key("u")
	if e.board[3][1] != 8 || e.board[0][1] != 8 {
		t.Fatal("undo didn't restore the board")
	}
}

func TestNoMoveNoSpawn(t *testing.T) {
	e := newTest(2)
	e.board = [N][N]int{{2, 4, 8, 16}}
	before := e.board
	e.Key("left")
	if e.board != before {
		t.Fatal("a move that changes nothing must not add a tile")
	}
}

func TestGameOver(t *testing.T) {
	e := newTest(3)
	e.board = [N][N]int{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 0},
	}
	e.board[3][3] = 0
	// Sliding right fills the gap; whatever spawns, check consistency.
	for i := 0; i < 200 && e.state == solo.Playing; i++ {
		e.Key([]string{"left", "right", "up", "down"}[i%4])
	}
	if e.state == solo.Playing && !e.movesLeft() {
		t.Fatal("no moves left but still playing")
	}
}

func TestViewSize(t *testing.T) {
	v := newTest(4).View()
	if w, h := lipgloss.Width(v), lipgloss.Height(v); w != N*(tileW+1)+1 || h != N*4+1 {
		t.Fatalf("view is %dx%d", w, h)
	}
}
