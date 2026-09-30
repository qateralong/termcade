package chickenrun

import "math/rand/v2"

// Levels are 18 tiles tall; each tile is 2×2 pixels. They are generated from
// a fixed seed, so every level is always the same, and the level for a match
// is picked at random.
const (
	levelRows = 18
	startRun  = 24 // flat tiles at the start
	endRun    = 30 // flat tiles after the finish line
)

// levelSpec tunes the generator for one named level.
type levelSpec struct {
	name     string
	seed     uint64
	length   int // tiles before the finish line
	gaps     int // weight of floor and ceiling gaps
	pillars  int // weight of pillars
	floating int // weight of floating platforms
	speed    float64
}

var levels = []levelSpec{
	{name: "Barnyard", seed: 11, length: 300, gaps: 3, pillars: 3, floating: 1, speed: 1.0},
	{name: "Henhouse", seed: 23, length: 320, gaps: 2, pillars: 5, floating: 2, speed: 1.0},
	{name: "Windmill", seed: 37, length: 340, gaps: 4, pillars: 3, floating: 3, speed: 1.08},
	{name: "Egg Factory", seed: 51, length: 360, gaps: 4, pillars: 4, floating: 2, speed: 1.12},
	{name: "Fox Den", seed: 73, length: 380, gaps: 5, pillars: 5, floating: 3, speed: 1.18},
}

// level is a generated course.
type level struct {
	spec   levelSpec
	cols   int
	finish int      // finish line column, in tiles
	solid  [][]bool // [row][col]
}

func (l *level) at(col, row int) bool {
	if row < 0 || row >= levelRows || col < 0 {
		return false
	}
	if col >= l.cols {
		return row < 2 || row >= levelRows-2
	}
	return l.solid[row][col]
}

func buildLevel(spec levelSpec) *level {
	rng := rand.New(rand.NewPCG(spec.seed, spec.seed*7+1))
	cols := startRun + spec.length + endRun
	l := &level{spec: spec, cols: cols, finish: startRun + spec.length}
	l.solid = make([][]bool, levelRows)
	for r := range l.solid {
		l.solid[r] = make([]bool, cols)
	}
	// Floor and ceiling, two tiles thick.
	for c := 0; c < cols; c++ {
		for _, r := range []int{0, 1, levelRows - 2, levelRows - 1} {
			l.solid[r][c] = true
		}
	}

	set := func(col, width, row, height int, v bool) {
		for c := col; c < col+width && c < l.finish; c++ {
			for r := row; r < row+height; r++ {
				if r >= 0 && r < levelRows {
					l.solid[r][c] = v
				}
			}
		}
	}

	total := spec.gaps*2 + spec.pillars*2 + spec.floating + 2
	c := startRun
	for c < l.finish-8 {
		pick := rng.IntN(total)
		switch {
		case pick < spec.gaps: // hole in the floor
			w := 3 + rng.IntN(3)
			set(c, w, levelRows-2, 2, false)
			c += w + 5 + rng.IntN(4)
		case pick < spec.gaps*2: // hole in the ceiling
			w := 3 + rng.IntN(3)
			set(c, w, 0, 2, false)
			c += w + 5 + rng.IntN(4)
		case pick < spec.gaps*2+spec.pillars: // pillar from the floor
			h := 3 + rng.IntN(5)
			set(c, 2, levelRows-2-h, h, true)
			c += 2 + 6 + rng.IntN(5)
		case pick < spec.gaps*2+spec.pillars*2: // pillar from the ceiling
			h := 3 + rng.IntN(5)
			set(c, 2, 2, h, true)
			c += 2 + 6 + rng.IntN(5)
		case pick < spec.gaps*2+spec.pillars*2+spec.floating: // floating ledge
			w := 5 + rng.IntN(6)
			row := 6 + rng.IntN(6)
			set(c, w, row, 1, true)
			c += w + 4 + rng.IntN(4)
		default: // breather
			c += 4 + rng.IntN(6)
		}
	}
	return l
}
