// Package bomber is a Bomberman-style brawl: drop bombs, blast crates for
// power-ups, and catch the others in the flames. Last one standing wins.
package bomber

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/ui/theme"
)

// The board is 15×9 tiles of 4×4 pixels, filling the standard arena.
const (
	Cols = 15
	Rows = 9
	tile = 4

	seats       = 4
	fuse        = 2500 * time.Millisecond
	flameTime   = 500 * time.Millisecond
	baseStep    = 190 * time.Millisecond // time per tile walked
	minStep     = 110 * time.Millisecond
	speedBonus  = 25 * time.Millisecond
	crateChance = 65 // percent of free tiles
	dropChance  = 30 // percent of crates that hide a power-up
	roundLength = 3 * time.Minute
)

// Game returns Bomberman.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "bomber",
			Name:        "Bomberman",
			Icon:        "◆",
			Tagline:     "Drop bombs, blast walls, trap your friends.",
			Description: "A classic maze brawl. Blow up crates to find power-ups for more bombs, bigger blasts and faster feet, chain explosions together, and corner your opponents until only one is left standing.",
			Kind:        games.Realtime,
			Players:     "2–4",
			Controls:    []string{"←↑→↓", "move", "space", "drop bomb"},
			Accent:      []string{theme.Coral, theme.Amber, theme.Pink},
			Art: []string{
				" ███████████████████████████",
				" █ ◉    ▒▒   ▒▒         ◉  █",
				" █ █ █ █ █ █▒█ █ █ █ █ █ █ █",
				" █   ▒▒    ━━━╋━━━   ▒▒    █",
				" ███████████████████████████",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng)
		},
	})
}

type cell byte

const (
	floor cell = iota
	wall
	crate
)

type power byte

const (
	noPower power = iota
	morebombs
	fire
	speed
)

type pt struct{ x, y int }

func (p pt) add(q pt) pt { return pt{p.x + q.x, p.y + q.y} }

var dirs = []pt{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

type player struct {
	info    multi.SeatInfo
	bot     bool
	pos     pt
	alive   bool
	outAt   time.Duration
	nextMv  time.Duration // when they may step again
	pending pt            // a buffered step
	bombs   int           // max bombs at once
	range_  int
	speed   int
	kills   int
	// bot state
	thinkAt time.Duration
}

type bomb struct {
	pos   pt
	owner int
	at    time.Duration // when it goes off
	power int
}

type flame struct {
	pos   pt
	until time.Duration
	owner int
}

// World is one match.
type World struct {
	rng     *rand.Rand
	grid    [Rows][Cols]cell
	powers  map[pt]power
	hidden  map[pt]power // power-ups inside crates
	players []*player
	bombs   []*bomb
	flames  []*flame
	clock   time.Duration
}

var spawnPoints = []pt{{1, 1}, {Cols - 2, Rows - 2}, {Cols - 2, 1}, {1, Rows - 2}}

// New builds a board and places the players in the corners.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng, powers: map[pt]power{}, hidden: map[pt]power{}}
	safe := map[pt]bool{}
	for _, s := range spawnPoints {
		for _, d := range []pt{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			safe[s.add(d)] = true
		}
	}
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			switch {
			case x == 0 || y == 0 || x == Cols-1 || y == Rows-1 || (x%2 == 0 && y%2 == 0):
				w.grid[y][x] = wall
			case !safe[pt{x, y}] && rng.IntN(100) < crateChance:
				w.grid[y][x] = crate
				if rng.IntN(100) < dropChance {
					w.hidden[pt{x, y}] = power(1 + rng.IntN(3))
				}
			}
		}
	}
	for i, s := range seatsInfo {
		w.players = append(w.players, &player{
			info: s, bot: s.Bot, pos: spawnPoints[i%len(spawnPoints)], alive: true,
			bombs: 1, range_: 2,
		})
	}
	return w
}

func (w *World) stepTime(p *player) time.Duration {
	return max(minStep, baseStep-time.Duration(p.speed)*speedBonus)
}

func (w *World) bombAt(p pt) *bomb {
	for _, b := range w.bombs {
		if b.pos == p {
			return b
		}
	}
	return nil
}

// walkable reports whether a player may step onto p.
func (w *World) walkable(p pt) bool {
	return p.x >= 0 && p.y >= 0 && p.x < Cols && p.y < Rows && w.grid[p.y][p.x] == floor && w.bombAt(p) == nil
}

func (w *World) onFire(p pt) bool {
	for _, f := range w.flames {
		if f.pos == p {
			return true
		}
	}
	return false
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	p := w.players[seat]
	if !p.alive {
		return
	}
	switch key {
	case "up", "w", "k":
		w.walk(p, dirs[0])
	case "right", "d", "l":
		w.walk(p, dirs[1])
	case "down", "s", "j":
		w.walk(p, dirs[2])
	case "left", "a", "h":
		w.walk(p, dirs[3])
	case " ", "enter", "b":
		w.drop(seat)
	}
}

// walk steps now if the player is ready, or buffers the step.
func (w *World) walk(p *player, d pt) {
	if w.clock < p.nextMv {
		p.pending = d
		return
	}
	w.stepTo(p, d)
}

func (w *World) stepTo(p *player, d pt) {
	n := p.pos.add(d)
	if !w.walkable(n) {
		return
	}
	p.pos = n
	p.nextMv = w.clock + w.stepTime(p)
	if pw, ok := w.powers[n]; ok {
		delete(w.powers, n)
		switch pw {
		case morebombs:
			p.bombs = min(p.bombs+1, 6)
		case fire:
			p.range_ = min(p.range_+1, 7)
		case speed:
			p.speed = min(p.speed+1, 3)
		}
	}
}

func (w *World) drop(seat int) {
	p := w.players[seat]
	if !p.alive || w.bombAt(p.pos) != nil {
		return
	}
	n := 0
	for _, b := range w.bombs {
		if b.owner == seat {
			n++
		}
	}
	if n >= p.bombs {
		return
	}
	w.bombs = append(w.bombs, &bomb{pos: p.pos, owner: seat, at: w.clock + fuse, power: p.range_})
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.players[seat].bot = true }

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	for i, p := range w.players {
		if !p.alive {
			continue
		}
		if p.bot {
			w.think(i, p)
		}
		if p.pending != (pt{}) && w.clock >= p.nextMv {
			d := p.pending
			p.pending = pt{}
			w.stepTo(p, d)
		}
	}

	// Bombs go off, setting off any bombs their flames reach.
	for {
		var due *bomb
		for _, b := range w.bombs {
			if w.clock >= b.at {
				due = b
				break
			}
		}
		if due == nil {
			break
		}
		w.explode(due)
	}

	kept := w.flames[:0]
	for _, f := range w.flames {
		if w.clock < f.until {
			kept = append(kept, f)
		}
	}
	w.flames = kept

	for _, f := range w.flames {
		for i, p := range w.players {
			if p.alive && p.pos == f.pos {
				p.alive, p.outAt = false, w.clock
				if f.owner != i {
					w.players[f.owner].kills++
				}
			}
		}
	}
}

// blast lists the tiles a bomb at pos with the given power would burn.
func (w *World) blast(pos pt, pow int) []pt {
	out := []pt{pos}
	for _, d := range dirs {
		p := pos
		for k := 0; k < pow; k++ {
			p = p.add(d)
			c := w.grid[p.y][p.x]
			if c == wall {
				break
			}
			out = append(out, p)
			if c == crate {
				break
			}
		}
	}
	return out
}

func (w *World) explode(b *bomb) {
	for i, x := range w.bombs {
		if x == b {
			w.bombs = append(w.bombs[:i], w.bombs[i+1:]...)
			break
		}
	}
	for _, p := range w.blast(b.pos, b.power) {
		w.flames = append(w.flames, &flame{pos: p, until: w.clock + flameTime, owner: b.owner})
		switch {
		case w.grid[p.y][p.x] == crate:
			w.grid[p.y][p.x] = floor
			if pw, ok := w.hidden[p]; ok {
				delete(w.hidden, p)
				w.powers[p] = pw
			}
		default:
			delete(w.powers, p) // flames burn loose power-ups
		}
		if other := w.bombAt(p); other != nil {
			other.at = w.clock // chain reaction
		}
	}
}

func (w *World) aliveCount() int {
	n := 0
	for _, p := range w.players {
		if p.alive {
			n++
		}
	}
	return n
}

// Over implements multi.World.
func (w *World) Over() bool { return w.aliveCount() <= 1 || w.clock >= roundLength }

// Standings implements multi.World.
func (w *World) Standings() []multi.Standing {
	idx := make([]int, len(w.players))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := w.players[idx[a]], w.players[idx[b]]
		if pa.alive != pb.alive {
			return pa.alive
		}
		if !pa.alive && pa.outAt != pb.outAt {
			return pa.outAt > pb.outAt
		}
		return pa.kills > pb.kills
	})
	res := make([]multi.Standing, len(idx))
	for i, s := range idx {
		k := w.players[s].kills
		d := strconv.Itoa(k) + " kills"
		if k == 1 {
			d = "1 kill"
		}
		res[i] = multi.Standing{Seat: s, Detail: d}
	}
	return res
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	p := w.players[seat]
	return []multi.Stat{
		{Label: "BOMBS", Value: strconv.Itoa(p.bombs)},
		{Label: "FIRE", Value: strconv.Itoa(p.range_)},
		{Label: "SPEED", Value: strconv.Itoa(p.speed + 1)},
	}
}
