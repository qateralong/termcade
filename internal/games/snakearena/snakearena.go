// Package snakearena is multiplayer snake: everyone shares one arena, eats
// dots to score and grow, and tries not to crash into anyone.
package snakearena

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

const (
	W = multi.ArenaW
	H = multi.ArenaH

	seats       = 6
	roundLength = 2 * time.Minute
	moveEvery   = 85 * time.Millisecond
	respawnTime = 3 * time.Second
	startLen    = 5
	dotCount    = 30
	dotGrowth   = 2
	killBonus   = 5
	queueLimit  = 3
)

// Game returns Snake Arena.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "snake-arena",
			Name:        "Snake Arena",
			Icon:        "●",
			Tagline:     "Classic snake, but everyone's in the same pit.",
			Description: "Collect dots to score and grow while dodging the other snakes. Make someone crash into you for bonus points. Most points after two minutes wins.",
			Kind:        games.Realtime,
			Players:     "2–6",
			Controls:    []string{"←↑→↓", "turn", "wasd", "turn"},
			Accent:      []string{theme.Lime, theme.Mint, theme.Cyan},
			Art: []string{
				"    ·          ◆             ·",
				"  ●━━━━━━━━┓          ┏━━━━━━━●",
				"     ·     ┗━━━━━┓    ┃    ·",
				"   ◆             ┗━━━━┛        ◆",
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

type snake struct {
	info   multi.SeatInfo
	bot    bool
	body   []pt // head first
	dir    pt
	queue  []pt
	grow   int
	alive  bool
	downAt time.Duration // when it died, for the respawn timer
	score  int
	kills  int
}

// World is one round of Snake Arena.
type World struct {
	rng    *rand.Rand
	snakes []*snake
	dots   map[pt]bool
	clock  time.Duration
	acc    time.Duration
}

// New starts a round.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng, dots: map[pt]bool{}}
	for i, s := range seatsInfo {
		sn := &snake{info: s, bot: s.Bot}
		w.snakes = append(w.snakes, sn)
		w.spawn(sn, i)
	}
	for len(w.dots) < dotCount {
		w.addDot()
	}
	return w
}

// spawn places a snake. Initial spawns are spread around the arena; later
// ones pick a random open spot.
func (w *World) spawn(s *snake, slot int) {
	var head, dir pt
	if slot >= 0 {
		cols := []int{W / 6, W / 2, 5 * W / 6}
		rows := []int{H / 4, 3 * H / 4}
		head = pt{cols[slot%3], rows[(slot/3)%2]}
		dir = pt{1, 0}
		if slot%2 == 1 {
			dir = pt{-1, 0}
		}
	} else {
		occ := w.occupied()
		for tries := 0; tries < 200; tries++ {
			head = pt{4 + w.rng.IntN(W-8), 4 + w.rng.IntN(H-8)}
			dir = dirs[w.rng.IntN(4)]
			ok := true
			for i := -startLen; i <= startLen+4; i++ {
				for _, d := range []pt{{0, 0}, {dir.y, dir.x}, {-dir.y, -dir.x}} {
					p := pt{head.x + dir.x*i + d.x, head.y + dir.y*i + d.y}
					if occ[p] {
						ok = false
					}
				}
			}
			if ok {
				break
			}
		}
	}
	s.body = s.body[:0]
	for i := 0; i < startLen; i++ {
		s.body = append(s.body, pt{head.x - dir.x*i, head.y - dir.y*i})
	}
	s.dir, s.queue, s.grow, s.alive = dir, nil, 0, true
}

func (w *World) occupied() map[pt]bool {
	occ := map[pt]bool{}
	for _, s := range w.snakes {
		if s.alive {
			for _, b := range s.body {
				occ[b] = true
			}
		}
	}
	return occ
}

func (w *World) addDot() {
	occ := w.occupied()
	for tries := 0; tries < 100; tries++ {
		p := pt{1 + w.rng.IntN(W-2), 1 + w.rng.IntN(H-2)}
		if !occ[p] && !w.dots[p] {
			w.dots[p] = true
			return
		}
	}
}

func inside(p pt) bool { return p.x >= 0 && p.y >= 0 && p.x < W && p.y < H }

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	s := w.snakes[seat]
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
	default:
		return
	}
	last := s.dir
	if n := len(s.queue); n > 0 {
		last = s.queue[n-1]
	}
	if d == last || d == (pt{-last.x, -last.y}) || len(s.queue) >= queueLimit {
		return
	}
	s.queue = append(s.queue, d)
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.snakes[seat].bot = true }

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	for _, s := range w.snakes {
		if !s.alive && w.clock-s.downAt >= respawnTime {
			w.spawn(s, -1)
		}
	}
	w.acc += dt
	for w.acc >= moveEvery && !w.Over() {
		w.acc -= moveEvery
		w.move()
	}
}

func (w *World) move() {
	// Bots decide first, from the same state humans see.
	occ := w.occupied()
	for _, s := range w.snakes {
		if s.alive && s.bot {
			w.think(s, occ)
		}
	}

	// Everyone moves at once.
	heads := make([]pt, len(w.snakes))
	for i, s := range w.snakes {
		if !s.alive {
			continue
		}
		if len(s.queue) > 0 {
			s.dir, s.queue = s.queue[0], s.queue[1:]
		}
		heads[i] = s.body[0].add(s.dir)
	}
	// Bodies after tails move out of the way.
	bodyOwner := map[pt]int{}
	for i, s := range w.snakes {
		if !s.alive {
			continue
		}
		body := s.body
		if s.grow == 0 {
			body = body[:len(body)-1]
		}
		for _, b := range body {
			bodyOwner[b] = i
		}
	}
	headCount := map[pt]int{}
	for i, s := range w.snakes {
		if s.alive {
			headCount[heads[i]]++
		}
	}

	var dead []int
	for i, s := range w.snakes {
		if !s.alive {
			continue
		}
		h := heads[i]
		switch owner, hit := bodyOwner[h]; {
		case !inside(h):
			dead = append(dead, i)
		case headCount[h] > 1:
			dead = append(dead, i)
		case hit:
			dead = append(dead, i)
			if owner != i {
				w.snakes[owner].score += killBonus
				w.snakes[owner].kills++
			}
		}
	}
	isDead := map[int]bool{}
	for _, i := range dead {
		isDead[i] = true
	}

	for i, s := range w.snakes {
		if !s.alive {
			continue
		}
		if isDead[i] {
			w.kill(s)
			continue
		}
		s.body = append([]pt{heads[i]}, s.body...)
		if s.grow > 0 {
			s.grow--
		} else {
			s.body = s.body[:len(s.body)-1]
		}
		if w.dots[heads[i]] {
			delete(w.dots, heads[i])
			s.score++
			s.grow += dotGrowth
		}
	}
	for len(w.dots) < dotCount {
		w.addDot()
	}
}

// kill ends a snake's life: it drops dots where its body was and loses a
// quarter of its points.
func (w *World) kill(s *snake) {
	s.alive = false
	s.downAt = w.clock
	s.score -= s.score / 4
	for i, b := range s.body {
		if i%2 == 0 && inside(b) && len(w.dots) < dotCount*3 {
			w.dots[b] = true
		}
	}
}

// Over implements multi.World.
func (w *World) Over() bool { return w.clock >= roundLength }

// Standings implements multi.World.
func (w *World) Standings() []multi.Standing {
	idx := make([]int, len(w.snakes))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return w.snakes[idx[a]].score > w.snakes[idx[b]].score })
	out := make([]multi.Standing, len(idx))
	for i, s := range idx {
		out[i] = multi.Standing{Seat: s, Detail: strconv.Itoa(w.snakes[s].score) + " pts"}
	}
	return out
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	return []multi.Stat{
		{Label: "SCORE", Value: strconv.Itoa(w.snakes[seat].score)},
		{Label: "TIME", Value: multi.Clock(int((roundLength - w.clock + time.Second - 1) / time.Second))},
	}
}

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	c := canvas.New(W, H)
	for p := range w.dots {
		c.Set(p.x, p.y, "#F5C9B8")
	}
	for i, s := range w.snakes {
		if !s.alive {
			continue
		}
		body := s.info.Color
		head := theme.Mix(body, "#FFFFFF", 0.55)
		if i == seat {
			head = "#FFFFFF"
		}
		for j := len(s.body) - 1; j >= 0; j-- {
			col := body
			if j == 0 {
				col = head
			} else if j%2 == 1 {
				col = theme.Mix(body, "#000000", 0.18)
			}
			c.Set(s.body[j].x, s.body[j].y, col)
		}
	}
	me := w.snakes[seat]
	if !me.alive {
		left := (respawnTime - (w.clock - me.downAt) + time.Second - 1) / time.Second
		c.TextCenter(H/4, " respawning in "+strconv.Itoa(int(left))+" ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.snakes))
	for _, st := range w.Standings() {
		s := w.snakes[st.Seat]
		rows = append(rows, multi.SidebarRow{
			Name: s.info.Name, Color: s.info.Color, Value: strconv.Itoa(s.score),
			You: st.Seat == seat, Out: !s.alive,
		})
	}
	secs := int((roundLength - w.clock + time.Second - 1) / time.Second)
	return multi.Arena(th, c, "TIME LEFT", multi.Clock(secs), rows, "* is you", "crash into you:", "+5 points")
}
