package battleship

import "math/rand/v2"

// N is the board size.
const N = 10

// Fleet sizes, Russian rules: one 4, two 3s, three 2s and four 1s. Ships
// may not touch each other, not even at the corners.
var fleet = []int{4, 3, 3, 2, 2, 2, 1, 1, 1, 1}

// Cells of a board as its owner knows it.
const (
	water byte = iota
	ship
	miss // shot, nothing there
	hit  // shot, part of a ship that's still afloat
	sunk // shot, part of a sunk ship
)

type cell struct{ x, y int }

type board struct {
	grid  [N][N]byte
	ships [][]cell
}

func in(x, y int) bool { return x >= 0 && y >= 0 && x < N && y < N }

// randomBoard places the fleet at random, respecting the no-touch rule.
func randomBoard(rng *rand.Rand) *board {
	for {
		b := &board{}
		ok := true
		for _, size := range fleet {
			placed := false
			for tries := 0; tries < 200 && !placed; tries++ {
				horiz := rng.IntN(2) == 0
				x, y := rng.IntN(N), rng.IntN(N)
				if b.canPlace(x, y, size, horiz) {
					b.place(x, y, size, horiz)
					placed = true
				}
			}
			if !placed {
				ok = false
				break
			}
		}
		if ok {
			return b
		}
	}
}

func (b *board) canPlace(x, y, size int, horiz bool) bool {
	for i := 0; i < size; i++ {
		cx, cy := x, y+i
		if horiz {
			cx, cy = x+i, y
		}
		if !in(cx, cy) {
			return false
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if in(cx+dx, cy+dy) && b.grid[cy+dy][cx+dx] == ship {
					return false
				}
			}
		}
	}
	return true
}

func (b *board) place(x, y, size int, horiz bool) {
	var s []cell
	for i := 0; i < size; i++ {
		cx, cy := x, y+i
		if horiz {
			cx, cy = x+i, y
		}
		b.grid[cy][cx] = ship
		s = append(s, cell{cx, cy})
	}
	b.ships = append(b.ships, s)
}

// shotResult describes what a shot did.
type shotResult int

const (
	resMiss shotResult = iota
	resHit
	resSunk
	resInvalid
)

// shoot fires at (x, y). Sinking a ship also marks the water around it as
// missed, since no ship can be there.
func (b *board) shoot(x, y int) shotResult {
	if !in(x, y) {
		return resInvalid
	}
	switch b.grid[y][x] {
	case water:
		b.grid[y][x] = miss
		return resMiss
	case ship:
		b.grid[y][x] = hit
	default:
		return resInvalid
	}
	for _, s := range b.ships {
		if !contains(s, x, y) {
			continue
		}
		for _, c := range s {
			if b.grid[c.y][c.x] != hit {
				return resHit
			}
		}
		for _, c := range s {
			b.grid[c.y][c.x] = sunk
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if in(c.x+dx, c.y+dy) && b.grid[c.y+dy][c.x+dx] == water {
						b.grid[c.y+dy][c.x+dx] = miss
					}
				}
			}
		}
		return resSunk
	}
	return resHit
}

func contains(s []cell, x, y int) bool {
	for _, c := range s {
		if c.x == x && c.y == y {
			return true
		}
	}
	return false
}

// afloat counts ship cells not yet hit.
func (b *board) afloat() int {
	n := 0
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			if b.grid[y][x] == ship {
				n++
			}
		}
	}
	return n
}

// shipsLeft counts ships that aren't sunk.
func (b *board) shipsLeft() int {
	n := 0
	for _, s := range b.ships {
		if b.grid[s[0].y][s[0].x] != sunk {
			n++
		}
	}
	return n
}

// known is what the opponent can see: ships hide as water.
func (b *board) known(x, y int) byte {
	if v := b.grid[y][x]; v != ship {
		return v
	}
	return water
}
