package battleship

// aim picks a target on the enemy board: finish off a wounded ship along its
// line, otherwise hunt on a checkerboard sized for the biggest ship left.
func (w *World) aim(seat int) (int, int) {
	b := w.p[1-seat].board
	unknown := func(x, y int) bool { return in(x, y) && b.known(x, y) == water }

	var hits []cell
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			if b.known(x, y) == hit {
				hits = append(hits, cell{x, y})
			}
		}
	}

	// Target mode.
	if len(hits) > 0 {
		var cands []cell
		if len(hits) >= 2 {
			horiz := hits[0].y == hits[1].y
			lo, hi := hits[0], hits[0]
			for _, h := range hits {
				if horiz && h.x < lo.x || !horiz && h.y < lo.y {
					lo = h
				}
				if horiz && h.x > hi.x || !horiz && h.y > hi.y {
					hi = h
				}
			}
			if horiz {
				cands = []cell{{lo.x - 1, lo.y}, {hi.x + 1, hi.y}}
			} else {
				cands = []cell{{lo.x, lo.y - 1}, {hi.x, hi.y + 1}}
			}
		} else {
			h := hits[0]
			cands = []cell{{h.x + 1, h.y}, {h.x - 1, h.y}, {h.x, h.y + 1}, {h.x, h.y - 1}}
		}
		var ok []cell
		for _, c := range cands {
			if unknown(c.x, c.y) {
				ok = append(ok, c)
			}
		}
		if len(ok) > 0 {
			c := ok[w.rng.IntN(len(ok))]
			return c.x, c.y
		}
	}

	// Hunt mode: the biggest ship left decides the checkerboard spacing.
	biggest := 1
	for _, s := range b.ships {
		if b.grid[s[0].y][s[0].x] != sunk && len(s) > biggest {
			biggest = len(s)
		}
	}
	var cands []cell
	if w.rng.IntN(3) == 0 {
		biggest = 1 // a careless shot anywhere
	}
	offset := w.rng.IntN(biggest)
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			if unknown(x, y) && (x+y+offset)%biggest == 0 {
				cands = append(cands, cell{x, y})
			}
		}
	}
	if len(cands) == 0 {
		for y := 0; y < N; y++ {
			for x := 0; x < N; x++ {
				if unknown(x, y) {
					cands = append(cands, cell{x, y})
				}
			}
		}
	}
	if len(cands) == 0 {
		return 0, 0
	}
	c := cands[w.rng.IntN(len(cands))]
	return c.x, c.y
}
