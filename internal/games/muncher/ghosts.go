package muncher

import "time"

type ghostMode int

const (
	ghostWaiting ghostMode = iota // in the house
	ghostLeaving                  // walking out through the door
	ghostActive                   // roaming the maze
	ghostEaten                    // eyes heading home
)

type ghost struct {
	mover
	name       string
	color      string
	mode       ghostMode
	frightened bool
	reverse    bool
	corner     point // scatter target

	// Release rules for the first round of a level, and after a death.
	releaseDots  int
	releaseAfter time.Duration
}

func newGhosts() []*ghost {
	return []*ghost{
		{name: "Chaser", color: "#FF4F5E", mover: mover{pos: ghostExit, dir: dirLeft}, mode: ghostActive,
			corner: point{MazeW - 3, -3}},
		{name: "Ambusher", color: "#FF8BD8", mover: mover{pos: point{13, 10}, dir: dirUp}, mode: ghostWaiting,
			corner: point{2, -3}, releaseAfter: time.Second},
		{name: "Flanker", color: "#3DE8FF", mover: mover{pos: point{11, 10}, dir: dirUp}, mode: ghostWaiting,
			corner: point{MazeW - 1, MazeH + 1}, releaseDots: 30, releaseAfter: 6 * time.Second},
		{name: "Drifter", color: "#FFB347", mover: mover{pos: point{16, 10}, dir: dirUp}, mode: ghostWaiting,
			corner: point{0, MazeH + 1}, releaseDots: 60, releaseAfter: 11 * time.Second},
	}
}

// releaseGhosts lets waiting ghosts out based on dots eaten and time, and
// always lets one out if the muncher stops eating for a while.
func (e *Engine) releaseGhosts() {
	for _, g := range e.ghosts {
		if g.mode != ghostWaiting {
			continue
		}
		byDots := e.firstRound && g.releaseDots > 0 && e.eatenDots >= g.releaseDots
		byTime := e.roundTime >= g.releaseAfter
		if byDots || byTime || e.lastDot >= 4*time.Second {
			g.mode = ghostLeaving
			e.lastDot = 0
		}
		return // one at a time, in order
	}
}

// passable reports whether a ghost may enter p.
func (e *Engine) passable(g *ghost, p point) bool {
	switch e.mz.at(p) {
	case tileWall:
		return false
	case tileDoor, tileHouse:
		return g.mode == ghostLeaving || g.mode == ghostEaten
	}
	return true
}

// target is where the ghost is heading right now.
func (e *Engine) target(g *ghost) point {
	switch g.mode {
	case ghostLeaving:
		return ghostExit
	case ghostEaten:
		return houseSpot
	}
	if e.scattering() {
		return g.corner
	}
	pac, facing := e.pac.pos, e.pac.dir.vec()
	switch g.name {
	case "Ambusher": // aims four cells ahead of the muncher
		return point{pac.x + 4*facing.x, pac.y + 4*facing.y}
	case "Flanker": // uses Chaser to pincer the muncher
		pivot := point{pac.x + 2*facing.x, pac.y + 2*facing.y}
		b := e.ghosts[0].pos
		return point{2*pivot.x - b.x, 2*pivot.y - b.y}
	case "Drifter": // chases, but loses his nerve up close
		if dist2(g.pos, pac) < 64 {
			return g.corner
		}
	}
	return pac
}

func (e *Engine) moveGhost(g *ghost) {
	if g.mode == ghostWaiting {
		return
	}
	var choice dir
	if g.reverse && g.mode == ghostActive {
		g.reverse = false
		if r := g.dir.reverse(); e.passable(g, step(g.pos, r)) {
			choice = r
		}
	}
	if choice == dirNone {
		var options []dir
		for _, d := range ghostOrder {
			// Ghosts never turn around on their own, except in the house.
			if d == g.dir.reverse() && g.mode != ghostLeaving {
				continue
			}
			if e.passable(g, step(g.pos, d)) {
				options = append(options, d)
			}
		}
		switch {
		case len(options) == 0:
			choice = g.dir.reverse()
		case g.frightened && g.mode == ghostActive:
			choice = options[e.rng.IntN(len(options))]
		default:
			t := e.target(g)
			best := -1
			for _, d := range options {
				if dd := dist2(step(g.pos, d), t); best < 0 || dd < best {
					best, choice = dd, d
				}
			}
		}
	}
	if !e.passable(g, step(g.pos, choice)) {
		return
	}
	g.dir = choice
	g.pos = step(g.pos, choice)

	switch {
	case g.mode == ghostLeaving && g.pos.y <= ghostExit.y:
		g.mode, g.dir = ghostActive, dirLeft
		g.frightened = false
	case g.mode == ghostEaten && g.pos == houseSpot:
		g.mode = ghostLeaving
	}
}
