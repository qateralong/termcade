package snakearena

// think picks a direction for a bot snake: head for the nearest dot, but
// never into a wall or body, avoid cells next to other heads, and prefer
// moves that leave plenty of room.
func (w *World) think(s *snake, occ map[pt]bool) {
	head := s.body[0]
	target, best := pt{}, -1
	for d := range w.dots {
		if dist := abs(d.x-head.x) + abs(d.y-head.y); best < 0 || dist < best {
			target, best = d, dist
		}
	}

	danger := map[pt]bool{}
	for _, o := range w.snakes {
		if o != s && o.alive {
			for _, d := range dirs {
				danger[o.body[0].add(d)] = true
			}
		}
	}

	need := min(len(s.body)*2, 60)
	bestScore, choice := -1<<30, s.dir
	for _, d := range dirs {
		if d == (pt{-s.dir.x, -s.dir.y}) {
			continue
		}
		n := head.add(d)
		if !inside(n) || occ[n] {
			continue
		}
		score := 0
		if room := w.room(n, occ, need); room < need {
			score -= (need - room) * 50
		}
		if danger[n] {
			score -= 400
		}
		if best >= 0 {
			score -= (abs(target.x-n.x) + abs(target.y-n.y)) * 4
		}
		if d == s.dir {
			score += 2 // slight preference for going straight
		}
		score += w.rng.IntN(3)
		if score > bestScore {
			bestScore, choice = score, d
		}
	}
	s.queue = nil
	s.dir = choice
}

// room counts free cells reachable from start, up to limit.
func (w *World) room(start pt, occ map[pt]bool, limit int) int {
	seen := map[pt]bool{start: true}
	queue := []pt{start}
	for len(queue) > 0 && len(seen) < limit {
		p := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			n := p.add(d)
			if inside(n) && !occ[n] && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return len(seen)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
