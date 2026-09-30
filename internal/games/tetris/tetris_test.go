package tetris

import (
	"io"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

func newTest(seed uint64) *Engine {
	return New(theme.New(lipgloss.NewRenderer(io.Discard)), rand.New(rand.NewPCG(seed, seed)))
}

func TestRotationsHaveFourCells(t *testing.T) {
	for k := kind(0); k < numKinds; k++ {
		for r := 0; r < 4; r++ {
			seen := map[cellPos]bool{}
			for _, c := range shapes[k][r] {
				if seen[c] || c.x < 0 || c.y < 0 || c.x >= boxSize[k] || c.y >= boxSize[k] {
					t.Fatalf("piece %d rot %d has a bad cell %v", k, r, c)
				}
				seen[c] = true
			}
		}
		// Four clockwise turns return to the start.
		if shapes[k][0] != spawnShapes[k] {
			t.Fatalf("piece %d spawn shape changed", k)
		}
	}
}

func TestBagDealsEveryPiece(t *testing.T) {
	e := newTest(1)
	e.bag = nil
	seen := map[kind]int{}
	for i := 0; i < 14; i++ {
		seen[e.draw()]++
	}
	for k := kind(0); k < numKinds; k++ {
		if seen[k] != 2 {
			t.Fatalf("piece %d dealt %d times in two bags", k, seen[k])
		}
	}
}

func TestLineClearScoring(t *testing.T) {
	e := newTest(2)
	// Fill the bottom row except its first four cells, then drop a flat I
	// into the gap.
	bottom := allRows - 1
	for x := 4; x < Cols; x++ {
		e.board[bottom][x] = 1
	}
	e.cur = piece{k: pieceI, x: 0, y: 0}
	e.hardDrop()
	if len(e.clearing) != 1 {
		t.Fatalf("clearing %v", e.clearing)
	}
	e.Update(clearFlash)
	if e.lines != 1 || e.score < lineScores[1] {
		t.Fatalf("lines=%d score=%d", e.lines, e.score)
	}
	for x := 0; x < Cols; x++ {
		if e.board[bottom][x] != 0 {
			t.Fatal("bottom row not cleared")
		}
	}
}

func TestWallKickNearWall(t *testing.T) {
	e := newTest(3)
	// A vertical I against the left wall must kick out to rotate back.
	e.cur = piece{k: pieceI, rot: 1, x: -2, y: 5}
	if !e.fits(e.cur) {
		t.Fatal("setup: vertical I should fit against the wall")
	}
	if !e.rotate(1) {
		t.Fatal("rotation against the wall should kick")
	}
	for _, c := range e.cur.cells() {
		if c.x < 0 {
			t.Fatal("kicked into the wall")
		}
	}
}

func TestLockDelayAndGameOver(t *testing.T) {
	e := newTest(4)
	for e.fall() {
	}
	e.Update(lockDelay - time.Millisecond)
	if !e.active {
		t.Fatal("locked before the delay ran out")
	}
	e.Update(2 * time.Millisecond)
	if e.active && e.cur.y > 10 {
		t.Fatal("piece didn't lock after the delay")
	}

	// Fill the spawn area so the next piece can't appear.
	for y := 0; y < 4; y++ {
		for x := 0; x < Cols; x++ {
			e.board[y][x] = 1
		}
	}
	e.spawn(pieceT)
	if e.state != solo.Lost {
		t.Fatal("blocked spawn should end the game")
	}
}

func TestHold(t *testing.T) {
	e := newTest(5)
	first := e.cur.k
	e.Key("c")
	if !e.hasHold || e.hold != first {
		t.Fatal("piece not held")
	}
	second := e.cur.k
	e.Key("c") // only once per piece
	if e.cur.k != second {
		t.Fatal("held twice in a row")
	}
}

func TestViewSize(t *testing.T) {
	e := newTest(6)
	v := e.View()
	if h := lipgloss.Height(v); h != Rows+2 {
		t.Fatalf("view height %d", h)
	}
	if w := lipgloss.Width(v); w != sideW+1+Cols*2+2+1+sideW {
		t.Fatalf("view width %d", w)
	}
}

func TestMiniPiecesFit(t *testing.T) {
	e := newTest(7)
	for k := kind(0); k < numKinds; k++ {
		if w := lipgloss.Width(e.mini(k, true, false)); w != 8 {
			t.Errorf("piece %d preview is %d wide", k, w)
		}
	}
}
