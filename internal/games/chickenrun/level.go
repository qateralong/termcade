package chickenrun

import (
	"math"
	"math/rand/v2"
	"time"
)

// Levels are 18 tiles tall; each tile is 2×2 pixels. They are generated from
// a fixed seed, so every level is always the same, and the level for a match
// is picked at random.
const (
	levelRows = 18
	startRun  = 24 // flat tiles at the start
	endRun    = 30 // flat tiles after the finish line
)

// levelSpec tunes the generator for one named level. The obstacle fields
// are relative weights.
type levelSpec struct {
	name     string
	seed     uint64
	length   int // tiles before the finish line
	gaps     int // holes in the floor or ceiling
	pillars  int // pillars from the floor or ceiling
	floating int // floating ledges
	spikes   int // spike strips on the floor or ceiling
	saws     int // spiked blocks hanging in mid-air
	zigzags  int // a floor pillar then a ceiling pillar right after
	crushers int // blocks sliding between floor and ceiling
	speed    float64
}

var levels = []levelSpec{
	{name: "Barnyard", seed: 11, length: 300, gaps: 3, pillars: 3, floating: 1, spikes: 2, saws: 0, zigzags: 1, crushers: 0, speed: 1.0},
	{name: "Henhouse", seed: 23, length: 320, gaps: 2, pillars: 4, floating: 2, spikes: 2, saws: 1, zigzags: 2, crushers: 1, speed: 1.0},
	{name: "Windmill", seed: 37, length: 340, gaps: 3, pillars: 2, floating: 2, spikes: 2, saws: 2, zigzags: 1, crushers: 3, speed: 1.06},
	{name: "Egg Factory", seed: 51, length: 360, gaps: 3, pillars: 3, floating: 1, spikes: 3, saws: 2, zigzags: 2, crushers: 2, speed: 1.1},
	{name: "Fox Den", seed: 73, length: 380, gaps: 4, pillars: 3, floating: 2, spikes: 3, saws: 3, zigzags: 3, crushers: 2, speed: 1.15},
}

// mover is a solid block sliding up and down between two rows.
type mover struct {
	col, width, height int     // in tiles
	top0, top1         float64 // the top row at either end of its travel
	period             time.Duration
	phase              float64
}

// top returns the mover's top row at time t.
func (m mover) top(t time.Duration) float64 {
	k := 0.5 - 0.5*math.Cos(2*math.Pi*(t.Seconds()/m.period.Seconds()+m.phase))
	return m.top0 + (m.top1-m.top0)*k
}

// level is a generated course.
type level struct {
	spec   levelSpec
	cols   int
	finish int      // finish line column, in tiles
	solid  [][]bool // [row][col]
	spike  [][]bool // deadly to touch
	movers []mover
}

// at reports whether a tile is solid ground (movers aside).
func (l *level) at(col, row int) bool {
	if row < 0 || row >= levelRows || col < 0 {
		return false
	}
	if col >= l.cols {
		return row < 2 || row >= levelRows-2
	}
	return l.solid[row][col]
}

func (l *level) spikeAt(col, row int) bool {
	if row < 0 || row >= levelRows || col < 0 || col >= l.cols {
		return false
	}
	return l.spike[row][col]
}

// moverAt reports whether a mover covers the pixel at time t.
func (l *level) moverAt(px, py int, t time.Duration) bool {
	col := px / tile
	for _, m := range l.movers {
		if col < m.col || col >= m.col+m.width {
			continue
		}
		top := m.top(t) * tile
		if float64(py) >= top && float64(py) < top+float64(m.height*tile) {
			return true
		}
	}
	return false
}

func buildLevel(spec levelSpec) *level {
	rng := rand.New(rand.NewPCG(spec.seed, spec.seed*7+1))
	cols := startRun + spec.length + endRun
	l := &level{spec: spec, cols: cols, finish: startRun + spec.length}
	l.solid = make([][]bool, levelRows)
	l.spike = make([][]bool, levelRows)
	for r := range l.solid {
		l.solid[r] = make([]bool, cols)
		l.spike[r] = make([]bool, cols)
	}
	// Floor and ceiling, two tiles thick.
	for c := 0; c < cols; c++ {
		for _, r := range []int{0, 1, levelRows - 2, levelRows - 1} {
			l.solid[r][c] = true
		}
	}

	set := func(grid [][]bool, col, width, row, height int, v bool) {
		for c := col; c < col+width && c < l.finish; c++ {
			for r := row; r < row+height; r++ {
				if r >= 0 && r < levelRows {
					grid[r][c] = v
				}
			}
		}
	}

	weights := []int{spec.gaps, spec.gaps, spec.pillars, spec.pillars, spec.floating,
		spec.spikes, spec.saws, spec.zigzags, spec.crushers, 2}
	total := 0
	for _, w := range weights {
		total += w
	}
	pick := func() int {
		n := rng.IntN(total)
		for i, w := range weights {
			if n < w {
				return i
			}
			n -= w
		}
		return len(weights) - 1
	}

	c := startRun
	for c < l.finish-12 {
		switch pick() {
		case 0: // hole in the floor
			w := 3 + rng.IntN(3)
			set(l.solid, c, w, levelRows-2, 2, false)
			c += w + 5 + rng.IntN(4)
		case 1: // hole in the ceiling
			w := 3 + rng.IntN(3)
			set(l.solid, c, w, 0, 2, false)
			c += w + 5 + rng.IntN(4)
		case 2: // pillar from the floor
			h := 3 + rng.IntN(5)
			set(l.solid, c, 2, levelRows-2-h, h, true)
			c += 2 + 6 + rng.IntN(5)
		case 3: // pillar from the ceiling
			h := 3 + rng.IntN(5)
			set(l.solid, c, 2, 2, h, true)
			c += 2 + 6 + rng.IntN(5)
		case 4: // floating ledge
			w := 5 + rng.IntN(6)
			row := 6 + rng.IntN(6)
			set(l.solid, c, w, row, 1, true)
			c += w + 4 + rng.IntN(4)
		case 5: // spikes along the floor or the ceiling
			w := 3 + rng.IntN(3)
			row := levelRows - 3
			if rng.IntN(2) == 0 {
				row = 2
			}
			set(l.spike, c, w, row, 1, true)
			c += w + 6 + rng.IntN(4)
		case 6: // a spiked block hanging in the middle
			row := 7 + rng.IntN(3)
			set(l.spike, c, 2, row, 2, true)
			c += 2 + 7 + rng.IntN(4)
		case 7: // zigzag: floor pillar, then a ceiling pillar soon after
			h1, h2 := 4+rng.IntN(3), 4+rng.IntN(3)
			set(l.solid, c, 2, levelRows-2-h1, h1, true)
			gap := 6 + rng.IntN(3)
			set(l.solid, c+2+gap, 2, 2, h2, true)
			c += 2 + gap + 2 + 7 + rng.IntN(4)
		case 8: // a crusher sliding between floor and ceiling
			h := 7
			l.movers = append(l.movers, mover{
				col: c, width: 2, height: h,
				top0: 2, top1: float64(levelRows - 2 - h),
				period: time.Duration(2200+rng.IntN(900)) * time.Millisecond,
				phase:  rng.Float64(),
			})
			c += 2 + 8 + rng.IntN(4)
		default: // breather
			c += 4 + rng.IntN(6)
		}
	}
	return l
}
