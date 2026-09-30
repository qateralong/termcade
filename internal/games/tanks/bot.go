package tanks

import "time"

// Bots react a little slower than a person and don't always pull the
// trigger, so humans have a fighting chance.
const (
	thinkEvery   = 280 * time.Millisecond
	fireChance   = 55 // percent, when an enemy is in the line of fire
	noticeRadius = 26 // pixels; enemies farther away are hunted, not shot
)

// think drives a bot tank: shoot at an enemy in the line of fire, otherwise
// follow a path toward the nearest enemy, shooting bricks out of the way.
func (w *World) think(i int, t *tank) {
	if w.clock < t.thinkAt {
		return
	}
	t.thinkAt = w.clock + thinkEvery + time.Duration(w.rng.IntN(120))*time.Millisecond

	target := w.nearestEnemy(i, t)
	if target == nil {
		return
	}

	// Enemy in the line of fire? Turn and (usually) shoot.
	near := abs(target.x-t.x)+abs(target.y-t.y) <= noticeRadius
	if d, ok := w.lineOfFire(t, target); ok && near {
		t.dir = d
		t.fire = w.rng.IntN(100) < fireChance
		if w.rng.IntN(3) == 0 {
			t.drive = 0 // stand and shoot
		}
		return
	}

	// Stuck against something? Shoot it (bricks break) and sometimes turn.
	if t.stuck > 3 {
		t.fire = true
		if w.rng.IntN(3) == 0 {
			w.steer(t, dir(w.rng.IntN(4)))
			t.stuck = 0
			return
		}
	}

	if d, ok := w.pathStep(t, target); ok {
		w.steer(t, d)
		// Bricks straight ahead are in the way; blast them.
		if w.brickAhead(t) {
			t.fire = true
		}
		return
	}
	w.steer(t, dir(w.rng.IntN(4)))
}

func (w *World) nearestEnemy(i int, t *tank) *tank {
	var best *tank
	bestD := 1 << 30
	for j, o := range w.tanks {
		if j == i || !o.alive {
			continue
		}
		if d := abs(o.x-t.x) + abs(o.y-t.y); d < bestD {
			best, bestD = o, d
		}
	}
	return best
}

// lineOfFire reports the direction to shoot if target is lined up with t and
// nothing solid but bricks and bushes is in between.
func (w *World) lineOfFire(t, target *tank) (dir, bool) {
	cx, cy := t.x+1, t.y+1
	overlapX := t.x < target.x+tankSize-1 && target.x < t.x+tankSize-1
	overlapY := t.y < target.y+tankSize-1 && target.y < t.y+tankSize-1
	var d dir
	switch {
	case overlapX && target.y < t.y:
		d = up
	case overlapX && target.y > t.y:
		d = down
	case overlapY && target.x < t.x:
		d = left
	case overlapY && target.x > t.x:
		d = right
	default:
		return 0, false
	}
	dx, dy := d.vec()
	for x, y := cx+dx, cy+dy; x >= 0 && y >= 0 && x < W && y < H; x, y = x+dx, y+dy {
		if x >= target.x && x < target.x+tankSize && y >= target.y && y < target.y+tankSize {
			return d, true
		}
		if w.pix[y][x] == steel {
			return 0, false
		}
	}
	return 0, false
}

func (w *World) brickAhead(t *tank) bool {
	dx, dy := t.dir.vec()
	for k := 1; k <= 3; k++ {
		for o := 0; o < tankSize; o++ {
			x, y := t.x+1+dx*(1+k), t.y+1+dy*(1+k)
			if dx == 0 {
				x = t.x + o
				y = t.y + 1 + dy*(1+k)
				if dy > 0 {
					y = t.y + tankSize - 1 + k
				} else {
					y = t.y - k
				}
			} else {
				y = t.y + o
				if dx > 0 {
					x = t.x + tankSize - 1 + k
				} else {
					x = t.x - k
				}
			}
			if x >= 0 && y >= 0 && x < W && y < H && w.pix[y][x] == brick {
				return true
			}
		}
	}
	return false
}

// pathStep finds the first move of a shortest path toward target on the
// half-tile grid, treating bricks as passable (they can be shot through).
func (w *World) pathStep(t, target *tank) (dir, bool) {
	const cell = 2
	gw, gh := W/cell, H/cell
	passable := func(gx, gy int) bool {
		if gx < 0 || gy < 0 || gx+1 >= gw || gy+1 >= gh {
			return false
		}
		for y := gy * cell; y < gy*cell+tankSize; y++ {
			for x := gx * cell; x < gx*cell+tankSize; x++ {
				if m := w.pix[y][x]; m == steel || m == water {
					return false
				}
			}
		}
		return true
	}
	start := [2]int{(t.x + 1) / cell, (t.y + 1) / cell}
	goal := [2]int{(target.x + 1) / cell, (target.y + 1) / cell}
	prev := map[[2]int][2]int{start: start}
	queue := [][2]int{start}
	found := false
	for len(queue) > 0 && !found {
		p := queue[0]
		queue = queue[1:]
		for _, d := range []dir{up, right, down, left} {
			dx, dy := d.vec()
			n := [2]int{p[0] + dx, p[1] + dy}
			if _, seen := prev[n]; seen || !passable(n[0], n[1]) {
				continue
			}
			prev[n] = p
			if abs(n[0]-goal[0]) <= 1 && abs(n[1]-goal[1]) <= 1 {
				goal, found = n, true
				break
			}
			queue = append(queue, n)
		}
	}
	if !found {
		return 0, false
	}
	// Walk back to the first step.
	step := goal
	for prev[step] != start {
		step = prev[step]
	}
	switch {
	case step[0] > start[0]:
		return right, true
	case step[0] < start[0]:
		return left, true
	case step[1] > start[1]:
		return down, true
	case step[1] < start[1]:
		return up, true
	}
	return 0, false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
