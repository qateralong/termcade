package tron

// think steers a bot rider toward the direction with the most room, keeping
// away from other riders' heads.
func (w *World) think(r *rider, heads map[pt]bool) {
	if len(r.queue) > 0 {
		return
	}
	danger := map[pt]bool{}
	for h := range heads {
		if h == r.pos {
			continue
		}
		for _, d := range dirs {
			danger[h.add(d)] = true
		}
	}
	best, choice := -1<<30, r.dir
	for _, d := range dirs {
		if d == (pt{-r.dir.x, -r.dir.y}) {
			continue
		}
		n := r.pos.add(d)
		if !w.free(n) {
			continue
		}
		score := w.room(n, 160) * 10
		if danger[n] {
			score -= 2000
		}
		if d == r.dir {
			score += 15 // don't wiggle for nothing
		}
		score += w.rng.IntN(20)
		if score > best {
			best, choice = score, d
		}
	}
	r.dir = choice
	// Boost now and then when there's a long clear run ahead.
	if w.clock-r.boostAt >= boostCool && w.rng.IntN(40) == 0 {
		clear := 0
		for p := r.pos.add(r.dir); w.free(p) && clear < 20; p = p.add(r.dir) {
			clear++
		}
		if clear >= 15 {
			r.boostAt = w.clock
		}
	}
}

// room counts free cells reachable from start, up to limit.
func (w *World) room(start pt, limit int) int {
	seen := map[pt]bool{start: true}
	queue := []pt{start}
	for len(queue) > 0 && len(seen) < limit {
		p := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			n := p.add(d)
			if w.free(n) && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return len(seen)
}
