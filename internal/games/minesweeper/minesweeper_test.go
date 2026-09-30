package minesweeper

import (
	"io"
	"math/rand/v2"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(d Difficulty, seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), d, rand.New(rand.NewPCG(seed, seed)))
}

func TestFirstOpenIsSafe(t *testing.T) {
	for seed := uint64(0); seed < 50; seed++ {
		e := newTest(Difficulties["hard"], seed)
		e.open(0, 0)
		if e.state != solo.Playing {
			t.Fatalf("seed %d: first click hit a mine", seed)
		}
		if e.opened < 4 {
			t.Fatalf("seed %d: first click should open an area, opened %d", seed, e.opened)
		}
		mines := 0
		for _, c := range e.cells {
			if c.mine {
				mines++
			}
		}
		if mines != 99 {
			t.Fatalf("seed %d: %d mines", seed, mines)
		}
	}
}

func TestWinByOpeningEverything(t *testing.T) {
	e := newTest(Difficulties["easy"], 1)
	e.open(4, 4)
	e.Update(1500 * time.Millisecond)
	for y := 0; y < e.d.H; y++ {
		for x := 0; x < e.d.W; x++ {
			if !e.at(x, y).mine {
				e.open(x, y)
			}
		}
	}
	if e.state != solo.Won {
		t.Fatalf("state = %v", e.state)
	}
	if e.Score() != 1500 || e.flags != 10 {
		t.Fatalf("score=%d flags=%d", e.Score(), e.flags)
	}
	e.Update(time.Second)
	if e.Score() != 1500 {
		t.Fatal("clock kept running after the win")
	}
}

func TestMineLoses(t *testing.T) {
	e := newTest(Difficulties["easy"], 2)
	e.open(4, 4)
	for i, c := range e.cells {
		if c.mine {
			e.open(i%e.d.W, i/e.d.W)
			break
		}
	}
	if e.state != solo.Lost {
		t.Fatal("opening a mine didn't lose")
	}
}

func TestChordAndFlags(t *testing.T) {
	e := newTest(Difficulties["easy"], 3)
	e.open(4, 4)
	// Find an open number, flag its mines, and chord it.
	for y := 0; y < e.d.H; y++ {
		for x := 0; x < e.d.W; x++ {
			c := e.at(x, y)
			if !c.open || c.n == 0 {
				continue
			}
			e.neighbors(x, y, func(nx, ny int) {
				if e.at(nx, ny).mine {
					e.toggleFlag(nx, ny)
				}
			})
			e.open(x, y)
			if e.state != solo.Playing {
				t.Fatal("chording with correct flags hit a mine")
			}
			e.neighbors(x, y, func(nx, ny int) {
				if n := e.at(nx, ny); !n.mine && !n.open {
					t.Fatalf("neighbor (%d,%d) still closed after chord", nx, ny)
				}
			})
			return
		}
	}
	t.Fatal("no numbered cell found")
}

func TestMouseMapsToCells(t *testing.T) {
	e := newTest(Difficulties["easy"], 4)
	e.Mouse(1+2*3, 1+5, tea.MouseButtonLeft) // column 3, row 5
	if e.cx != 3 || e.cy != 5 || !e.at(3, 5).open {
		t.Fatalf("cursor (%d,%d), open=%v", e.cx, e.cy, e.at(3, 5).open)
	}
	e.Mouse(0, 0, tea.MouseButtonLeft) // on the border: ignored
}

func TestViewSizes(t *testing.T) {
	for key, d := range Difficulties {
		v := newTest(d, 5).View()
		if w, h := lipgloss.Width(v), lipgloss.Height(v); w != d.W*2+2 || h != d.H+2 {
			t.Errorf("%s: view is %dx%d", key, w, h)
		}
	}
}
