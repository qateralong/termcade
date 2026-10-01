// Package tron is light-cycle racing: every rider leaves a solid wall behind
// them, and the last one still riding wins.
package tron

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/games/solo"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

const (
	W = multi.ArenaW
	H = multi.ArenaH

	seats       = 6
	stepEvery   = 70 * time.Millisecond
	minStep     = 40 * time.Millisecond
	speedUpEach = 20 * time.Second // the whole field speeds up over time
	boostTime   = time.Second
	boostCool   = 4 * time.Second
	queueLimit  = 2
	roundLength = 3 * time.Minute
)

// Game returns Tron.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "tron",
			Name:        "Tron",
			Icon:        "◢",
			Tagline:     "Light cycles. Walls everywhere. Last one riding wins.",
			Description: "Every rider leaves a solid wall of light behind them. Cut others off, squeeze through gaps, and use your boost at the right moment. Crash into anything and you're out.",
			Kind:        games.Realtime,
			Players:     "2–6",
			Controls:    []string{"←↑→↓", "turn", "space", "boost"},
			Accent:      []string{theme.Cyan, theme.Sky, theme.Violet},
			Art: []string{
				"━━━━━━━━━━━━━━━━━━┓",
				"                  ┃    ┏━━━━━━━━━━▶",
				"   ◀━━━━━━━━━━━┓  ┗━━━━┛",
				"               ┃",
				"   ━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━╳",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng)
		},
	})
}

type pt struct{ x, y int }

func (p pt) add(q pt) pt { return pt{p.x + q.x, p.y + q.y} }

var dirs = []pt{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

type rider struct {
	info    multi.SeatInfo
	bot     bool
	pos     pt
	dir     pt
	queue   []pt
	alive   bool
	outAt   time.Duration
	boostAt time.Duration // when the last boost started
	kills   int
}

// World is one round of Tron.
type World struct {
	rng    *rand.Rand
	riders []*rider
	trail  [H][W]int8 // 0 empty, otherwise seat+1
	clock  time.Duration
	acc    time.Duration
	booms  []boom
}

type boom struct {
	p  pt
	at time.Duration
}

// New starts a round with riders spread around the arena, facing inward.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng}
	starts := []struct{ p, d pt }{
		{pt{6, 6}, pt{1, 0}}, {pt{W - 7, H - 7}, pt{-1, 0}},
		{pt{W - 7, 6}, pt{0, 1}}, {pt{6, H - 7}, pt{0, -1}},
		{pt{W / 2, 3}, pt{0, 1}}, {pt{W / 2, H - 4}, pt{0, -1}},
	}
	for i, s := range seatsInfo {
		st := starts[i%len(starts)]
		r := &rider{info: s, bot: s.Bot, pos: st.p, dir: st.d, alive: true, boostAt: -time.Hour}
		w.riders = append(w.riders, r)
		w.trail[st.p.y][st.p.x] = int8(i + 1)
	}
	return w
}

func inside(p pt) bool { return p.x >= 0 && p.y >= 0 && p.x < W && p.y < H }

func (w *World) free(p pt) bool { return inside(p) && w.trail[p.y][p.x] == 0 }

func (w *World) boosting(r *rider) bool { return w.clock-r.boostAt < boostTime }

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	r := w.riders[seat]
	if !r.alive {
		return
	}
	var d pt
	switch key {
	case "up", "w", "k":
		d = dirs[0]
	case "right", "d", "l":
		d = dirs[1]
	case "down", "s", "j":
		d = dirs[2]
	case "left", "a", "h":
		d = dirs[3]
	case " ", "enter":
		if w.clock-r.boostAt >= boostCool {
			r.boostAt = w.clock
		}
		return
	default:
		return
	}
	last := r.dir
	if n := len(r.queue); n > 0 {
		last = r.queue[n-1]
	}
	if d == last || d == (pt{-last.x, -last.y}) || len(r.queue) >= queueLimit {
		return
	}
	r.queue = append(r.queue, d)
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.riders[seat].bot = true }

func (w *World) interval() time.Duration {
	k := time.Duration(w.clock / speedUpEach)
	return max(minStep, stepEvery-k*8*time.Millisecond)
}

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	w.acc += dt
	for w.acc >= w.interval() && !w.Over() {
		w.acc -= w.interval()
		// Boosted riders move twice per step.
		w.move(func(r *rider) bool { return true })
		w.move(w.boosting)
	}
}

// move advances the riders selected by who by one cell, all at once.
func (w *World) move(who func(*rider) bool) {
	var movers []int
	occ := w.occupied()
	for i, r := range w.riders {
		if r.alive && who(r) {
			if r.bot {
				w.think(r, occ)
			}
			movers = append(movers, i)
		}
	}
	next := map[int]pt{}
	count := map[pt]int{}
	for _, i := range movers {
		r := w.riders[i]
		if len(r.queue) > 0 {
			r.dir, r.queue = r.queue[0], r.queue[1:]
		}
		n := r.pos.add(r.dir)
		next[i] = n
		count[n]++
	}
	for _, i := range movers {
		n := next[i]
		switch {
		case !inside(n):
			w.crash(i, -1)
		case count[n] > 1:
			w.crash(i, -1) // head-on
		case w.trail[n.y][n.x] != 0:
			w.crash(i, int(w.trail[n.y][n.x])-1)
		}
	}
	for _, i := range movers {
		r := w.riders[i]
		if !r.alive {
			continue
		}
		r.pos = next[i]
		w.trail[r.pos.y][r.pos.x] = int8(i + 1)
	}
}

func (w *World) occupied() map[pt]bool {
	occ := map[pt]bool{}
	for _, r := range w.riders {
		if r.alive {
			occ[r.pos] = true
		}
	}
	return occ
}

func (w *World) crash(i, into int) {
	r := w.riders[i]
	r.alive = false
	r.outAt = w.clock
	w.booms = append(w.booms, boom{r.pos, w.clock})
	if into >= 0 && into != i {
		w.riders[into].kills++
	}
}

func (w *World) aliveCount() int {
	n := 0
	for _, r := range w.riders {
		if r.alive {
			n++
		}
	}
	return n
}

// Over implements multi.World.
func (w *World) Over() bool { return w.aliveCount() <= 1 || w.clock >= roundLength }

// Standings implements multi.World: survivors first, then by how long each
// rider lasted.
func (w *World) Standings() []multi.Standing {
	idx := make([]int, len(w.riders))
	for i := range idx {
		idx[i] = i
	}
	lasted := func(r *rider) time.Duration {
		if r.alive {
			return time.Hour
		}
		return r.outAt
	}
	sort.SliceStable(idx, func(a, b int) bool { return lasted(w.riders[idx[a]]) > lasted(w.riders[idx[b]]) })
	res := make([]multi.Standing, len(idx))
	for i, s := range idx {
		r := w.riders[s]
		d := "survived"
		if !r.alive {
			d = solo.Duration(int(r.outAt / time.Millisecond))
		}
		res[i] = multi.Standing{Seat: s, Detail: d}
	}
	return res
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	r := w.riders[seat]
	boost := "ready"
	switch {
	case w.boosting(r):
		boost = "ON"
	case w.clock-r.boostAt < boostCool:
		boost = strconv.Itoa(int((boostCool-(w.clock-r.boostAt))/time.Second)+1) + "s"
	}
	return []multi.Stat{
		{Label: "ALIVE", Value: strconv.Itoa(w.aliveCount()) + "/" + strconv.Itoa(len(w.riders))},
		{Label: "BOOST", Value: boost},
	}
}

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	c := canvas.New(W, H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if s := w.trail[y][x]; s != 0 {
				r := w.riders[s-1]
				col := theme.Mix(r.info.Color, "#000000", 0.35)
				if !r.alive {
					col = theme.Mix(r.info.Color, "#000000", 0.7)
				}
				c.Set(x, y, col)
			} else if x%6 == 0 && y%6 == 0 {
				c.Set(x, y, "#2A2440") // a faint grid
			}
		}
	}
	for i, r := range w.riders {
		if !r.alive {
			continue
		}
		head := theme.Mix(r.info.Color, "#FFFFFF", 0.5)
		if i == seat {
			head = "#FFFFFF"
		}
		if w.boosting(r) && (w.clock/(80*time.Millisecond))%2 == 0 {
			head = theme.Amber
		}
		c.Set(r.pos.x, r.pos.y, head)
	}
	for _, b := range w.booms {
		age := w.clock - b.at
		if age > 600*time.Millisecond {
			continue
		}
		d := 1 + int(age/(150*time.Millisecond))
		for _, o := range []pt{{d, 0}, {-d, 0}, {0, d}, {0, -d}, {d, d}, {-d, -d}, {d, -d}, {-d, d}} {
			c.Set(b.p.x+o.x, b.p.y+o.y, "#FFE066")
		}
	}
	if !w.riders[seat].alive {
		c.TextCenter(1, " derezzed, watching ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.riders))
	for _, st := range w.Standings() {
		r := w.riders[st.Seat]
		v := "✓"
		if !r.alive {
			v = "✗"
		}
		rows = append(rows, multi.SidebarRow{Name: r.info.Name, Color: r.info.Color, Value: v, You: st.Seat == seat, Out: !r.alive})
	}
	return multi.Arena(th, c, "RIDING", strconv.Itoa(w.aliveCount())+" of "+strconv.Itoa(len(w.riders)), rows, "* is you", "space: boost")
}
