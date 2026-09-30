// Package pacman is a maze chase: eat every dot while four ghosts, each
// with its own personality, try to catch you.
package pacman

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

// Game returns the Pac-Man game.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "pacman",
			Name:        "Pac-Man",
			Icon:        "◖",
			Tagline:     "Eat the dots. Run from the ghosts. Then turn the tables.",
			Description: "Clear the maze while Blinky, Pinky, Inky and Clyde hunt you, each in their own way. Grab a power pellet and they're yours for a few seconds.",
			Kind:        games.Realtime,
			Players:     "1",
			Controls:    []string{"←↑→↓", "move", "wasd", "move"},
			Accent:      []string{theme.Amber, "#FF9F43", theme.Coral},
			Art: []string{
				"╭──────────────────────────╮",
				"│ ◖ · · · · ●   Ω  Ω   ·  │",
				"│ ╭────╮ · ╭────────╮ · ╭─╯",
				"│ ╰────╯ · ╰────────╯ · │",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 20 * time.Millisecond,
	})
}

type dir int

const (
	dirNone dir = iota
	dirUp
	dirLeft
	dirDown
	dirRight
)

// Tie-break order for ghost decisions, as in the arcade original.
var ghostOrder = []dir{dirUp, dirLeft, dirDown, dirRight}

func (d dir) vec() point {
	switch d {
	case dirUp:
		return point{0, -1}
	case dirDown:
		return point{0, 1}
	case dirLeft:
		return point{-1, 0}
	case dirRight:
		return point{1, 0}
	}
	return point{}
}

func (d dir) reverse() dir {
	switch d {
	case dirUp:
		return dirDown
	case dirDown:
		return dirUp
	case dirLeft:
		return dirRight
	case dirRight:
		return dirLeft
	}
	return dirNone
}

func step(p point, d dir) point {
	v := d.vec()
	return point{wrapX(p.x + v.x), p.y + v.y}
}

func dist2(a, b point) int {
	dx, dy := a.x-b.x, a.y-b.y
	return dx*dx + dy*dy
}

type phase int

const (
	phaseReady phase = iota
	phasePlay
	phaseDying
	phaseCleared
)

const (
	readyTime   = 2 * time.Second
	dyingTime   = 1500 * time.Millisecond
	clearedTime = 2 * time.Second
	fruitTime   = 9 * time.Second
	startLives  = 3
	extraLifeAt = 10000
	dotPts      = 10
	pelletPts   = 50
)

// Scatter/chase schedule; the last chase lasts forever.
var modeSchedule = []time.Duration{
	7 * time.Second, 20 * time.Second,
	7 * time.Second, 20 * time.Second,
	5 * time.Second, 20 * time.Second,
	5 * time.Second,
}

type mover struct {
	pos point
	dir dir
	acc float64 // progress toward the next cell
}

// Engine is one game of Pac-Man.
type Engine struct {
	th  *theme.Theme
	rng *rand.Rand
	mz  *maze

	walls [][]string

	pac  mover
	want dir
	chew bool // mouth animation

	ghosts []*ghost

	phase      phase
	phaseTime  time.Duration
	roundTime  time.Duration // since the round (life) began
	lastDot    time.Duration // time since a dot was last eaten
	modeIdx    int
	modeTime   time.Duration
	fright     time.Duration // remaining frightened time
	combo      int
	eatenDots  int
	firstRound bool

	fruit     bool
	fruitLeft time.Duration
	fruitsOut int

	score, lives, level int
	extraGiven          bool
	state               solo.State
	blink               time.Duration
}

// New starts a game.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, lives: startLives, level: 1}
	e.startLevel()
	return e
}

func (e *Engine) startLevel() {
	e.mz = newMaze()
	e.walls = wallGlyphs(e.mz)
	e.eatenDots = 0
	e.fruitsOut = 0
	e.firstRound = true
	e.startRound()
}

// startRound puts everyone back in place after a death or a new level.
func (e *Engine) startRound() {
	e.pac = mover{pos: pacStart, dir: dirLeft}
	e.want = dirLeft
	e.ghosts = newGhosts()
	e.phase, e.phaseTime = phaseReady, 0
	e.roundTime, e.lastDot = 0, 0
	e.modeIdx, e.modeTime = 0, 0
	e.fright, e.combo = 0, 0
	e.fruit = false
}

// ---------------------------------------------------------------------------
// Speeds, in cells per second.

func (e *Engine) pacSpeed() float64 {
	s := 7.5 + 0.25*float64(e.level-1)
	if e.fright > 0 {
		s += 0.5
	}
	return min(s, 10)
}

func (e *Engine) ghostSpeed(g *ghost) float64 {
	switch {
	case g.mode == ghostEaten:
		return 15
	case g.mode == ghostWaiting:
		return 0
	case g.mode == ghostLeaving:
		return 4
	case g.frightened:
		return 4.5
	case g.pos.y == 10 && (g.pos.x < 6 || g.pos.x >= MazeW-6): // tunnel
		return 4
	}
	return min(7+0.3*float64(e.level-1), 9.5)
}

func (e *Engine) frightTime() time.Duration {
	return max(time.Second, time.Duration(7-e.level)*time.Second)
}

func (e *Engine) scattering() bool {
	return e.modeIdx < len(modeSchedule) && e.modeIdx%2 == 0
}

// ---------------------------------------------------------------------------
// Simulation

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	if e.state != solo.Playing {
		return
	}
	e.blink += dt
	e.phaseTime += dt
	switch e.phase {
	case phaseReady:
		if e.phaseTime >= readyTime {
			e.phase = phasePlay
		}
		return
	case phaseDying:
		if e.phaseTime >= dyingTime {
			e.lives--
			if e.lives <= 0 {
				e.state = solo.Lost
				return
			}
			e.firstRound = false
			e.startRound()
		}
		return
	case phaseCleared:
		if e.phaseTime >= clearedTime {
			e.level++
			e.startLevel()
		}
		return
	}

	e.roundTime += dt
	e.lastDot += dt
	e.updateModes(dt)
	e.releaseGhosts()

	if e.fruit {
		e.fruitLeft -= dt
		if e.fruitLeft <= 0 {
			e.fruit = false
		}
	}

	secs := dt.Seconds()
	e.pac.acc += e.pacSpeed() * secs
	for e.pac.acc >= 1 && e.phase == phasePlay {
		e.pac.acc--
		e.movePac()
		e.collide()
	}
	for _, g := range e.ghosts {
		g.acc += e.ghostSpeed(g) * secs
		for g.acc >= 1 && e.phase == phasePlay {
			g.acc--
			e.moveGhost(g)
			e.collide()
		}
	}
}

func (e *Engine) updateModes(dt time.Duration) {
	if e.fright > 0 {
		e.fright -= dt
		if e.fright <= 0 {
			e.fright = 0
			for _, g := range e.ghosts {
				g.frightened = false
			}
		}
		return // the mode clock stops while ghosts are frightened
	}
	if e.modeIdx >= len(modeSchedule) {
		return
	}
	e.modeTime += dt
	if e.modeTime >= modeSchedule[e.modeIdx] {
		e.modeTime = 0
		e.modeIdx++
		for _, g := range e.ghosts {
			if g.mode == ghostActive {
				g.reverse = true
			}
		}
	}
}

func (e *Engine) movePac() {
	if e.want != dirNone && e.mz.open(step(e.pac.pos, e.want)) {
		e.pac.dir = e.want
	}
	next := step(e.pac.pos, e.pac.dir)
	if !e.mz.open(next) {
		e.pac.acc = 0 // stopped against a wall
		return
	}
	e.pac.pos = next
	e.chew = !e.chew
	e.eat()
}

func (e *Engine) eat() {
	p := e.pac.pos
	switch e.mz.dots[p.y][p.x] {
	case 1:
		e.addScore(dotPts)
	case 2:
		e.addScore(pelletPts)
		e.fright = e.frightTime()
		e.combo = 0
		for _, g := range e.ghosts {
			if g.mode == ghostActive {
				g.frightened = true
				g.reverse = true
			}
		}
	default:
		if e.fruit && p == pacStart {
			e.fruit = false
			e.addScore(e.fruitValue())
		}
		return
	}
	e.mz.dots[p.y][p.x] = 0
	e.eatenDots++
	e.lastDot = 0
	if e.eatenDots == 70 || e.eatenDots == 170 {
		e.fruit, e.fruitLeft = true, fruitTime
		e.fruitsOut++
	}
	if e.eatenDots == e.mz.total {
		e.phase, e.phaseTime = phaseCleared, 0
	}
}

func (e *Engine) fruitValue() int { return 100 * (e.level + 1) }

func (e *Engine) addScore(n int) {
	e.score += n
	if !e.extraGiven && e.score >= extraLifeAt {
		e.extraGiven = true
		e.lives++
	}
}

// collide resolves Pac-Man touching ghosts.
func (e *Engine) collide() {
	for _, g := range e.ghosts {
		if g.pos != e.pac.pos || g.mode != ghostActive {
			continue
		}
		if g.frightened {
			g.frightened = false
			g.mode = ghostEaten
			e.addScore(200 << min(e.combo, 3))
			e.combo++
			continue
		}
		e.phase, e.phaseTime = phaseDying, 0
		return
	}
}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	switch k {
	case "up", "w", "k":
		e.want = dirUp
	case "down", "s", "j":
		e.want = dirDown
	case "left", "a", "h":
		e.want = dirLeft
	case "right", "d", "l":
		e.want = dirRight
	}
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.score }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{
		{Label: "LIVES", Value: strings.Repeat("◖", max(e.lives, 0))},
		{Label: "LEVEL", Value: strconv.Itoa(e.level)},
	}
}
