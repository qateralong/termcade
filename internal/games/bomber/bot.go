package bomber

import "time"

const (
	botThink    = 320 * time.Millisecond
	botBlunders = 8 // percent chance to skip a sensible bomb or dawdle
)

// danger maps every tile that is burning or will burn before the bombs on
// the board go off, to the time it catches fire.
func (w *World) danger(extra *bomb) map[pt]time.Duration {
	d := map[pt]time.Duration{}
	for _, f := range w.flames {
		d[f.pos] = w.clock
	}
	bombs := append([]*bomb{}, w.bombs...)
	if extra != nil {
		bombs = append(bombs, extra)
	}
	for _, b := range bombs {
		for _, p := range w.blast(b.pos, b.power) {
			if t, ok := d[p]; !ok || b.at < t {
				d[p] = b.at
			}
		}
	}
	return d
}

// escape finds the first step toward the nearest tile that's safe from
// danger, considering how long walking there takes. ok is false if nowhere
// safe can be reached in time.
func (w *World) escape(p *player, danger map[pt]time.Duration) (pt, bool) {
	type node struct {
		pos   pt
		first pt
		steps int
	}
	step := w.stepTime(p)
	seen := map[pt]bool{p.pos: true}
	queue := []node{{pos: p.pos}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if _, bad := danger[n.pos]; !bad {
			return n.first, true
		}
		for _, d := range dirs {
			next := n.pos.add(d)
			if seen[next] || !w.walkable(next) {
				continue
			}
			arrive := w.clock + time.Duration(n.steps+1)*step
			if t, hot := danger[next]; hot && arrive >= t-50*time.Millisecond && arrive <= t+flameTime {
				continue // would be standing in the flames
			}
			seen[next] = true
			first := n.first
			if n.steps == 0 {
				first = d
			}
			queue = append(queue, node{next, first, n.steps + 1})
		}
	}
	return pt{}, false
}

// think decides a bot's next move: get out of danger first, then bomb
// crates or players when it can escape, otherwise head for a target.
func (w *World) think(seat int, p *player) {
	if w.clock < p.thinkAt {
		return
	}
	p.thinkAt = w.clock + botThink + time.Duration(w.rng.IntN(120))*time.Millisecond

	dz := w.danger(nil)
	if _, hot := dz[p.pos]; hot {
		if d, ok := w.escape(p, dz); ok && d != (pt{}) {
			w.walk(p, d)
		}
		return
	}

	if w.worthBombing(seat, p) && w.rng.IntN(100) >= botBlunders {
		test := &bomb{pos: p.pos, owner: seat, at: w.clock + fuse, power: p.range_}
		if _, ok := w.escape(p, w.danger(test)); ok {
			w.drop(seat)
			return
		}
	}

	if w.rng.IntN(100) < botBlunders {
		return // dawdle
	}
	if d, ok := w.towardTarget(seat, p, dz); ok {
		w.walk(p, d)
	}
}

// worthBombing reports whether a bomb here would hit a crate or a player.
func (w *World) worthBombing(seat int, p *player) bool {
	n := 0
	for _, b := range w.bombs {
		if b.owner == seat {
			n++
		}
	}
	if n >= p.bombs {
		return false
	}
	for _, t := range w.blast(p.pos, p.range_) {
		if w.grid[t.y][t.x] == crate {
			return true
		}
		for i, o := range w.players {
			if i != seat && o.alive && o.pos == t {
				return true
			}
		}
	}
	return false
}

// towardTarget steps along the shortest safe path to a power-up, a spot
// next to a crate, or an enemy, whichever is closest.
func (w *World) towardTarget(seat int, p *player, dz map[pt]time.Duration) (pt, bool) {
	isTarget := func(t pt) bool {
		if _, ok := w.powers[t]; ok {
			return true
		}
		for i, o := range w.players {
			if i != seat && o.alive && abs(o.pos.x-t.x)+abs(o.pos.y-t.y) <= 1 {
				return true
			}
		}
		for _, d := range dirs {
			n := t.add(d)
			if w.grid[n.y][n.x] == crate {
				return true
			}
		}
		return false
	}
	type node struct {
		pos   pt
		first pt
	}
	seen := map[pt]bool{p.pos: true}
	queue := []node{{pos: p.pos}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n.pos != p.pos && isTarget(n.pos) {
			return n.first, true
		}
		for _, d := range dirs {
			next := n.pos.add(d)
			if seen[next] || !w.walkable(next) {
				continue
			}
			if _, hot := dz[next]; hot {
				continue
			}
			seen[next] = true
			first := n.first
			if n.pos == p.pos {
				first = d
			}
			queue = append(queue, node{next, first})
		}
	}
	return pt{}, false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
