// Package battleship is the classic two-player naval duel, played by
// Russian rules: ships don't touch, and a hit earns another shot.
package battleship

import (
	"math/rand/v2"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/ui/theme"
)

const (
	placeTime = 30 * time.Second
	turnTime  = 25 * time.Second
	botDelay  = 900 * time.Millisecond
	showShot  = 600 * time.Millisecond // pause after a shot before the next turn
)

// Game returns Battleship.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "battleship",
			Name:        "Battleship",
			Icon:        "■",
			Tagline:     "Hide your fleet. Find theirs.",
			Description: "The timeless duel, by Russian rules: ships never touch, and every hit earns another shot. Arrange your fleet, then take turns calling shots until one fleet is sunk.",
			Kind:        games.TurnBased,
			Players:     "2",
			Controls:    []string{"←↑→↓", "aim", "enter", "fire", "r", "reshuffle fleet"},
			Accent:      []string{theme.Sky, theme.Cyan, theme.Mint},
			Art: []string{
				"    A B C D E F G H",
				"  1 · · ■ ■ ■ · · ·",
				"  2 · ╳ · · · · ○ ·",
				"  3 · · · ○ · · · ·",
			},
		},
		Seats: 2,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng)
		},
	})
}

type phase int

const (
	placing phase = iota
	battle
	over
)

type player struct {
	info   multi.SeatInfo
	bot    bool
	board  *board
	ready  bool
	cx, cy int // aiming cursor on the enemy board
	shots  int
	hits   int
}

// World is one match.
type World struct {
	rng     *rand.Rand
	p       [2]*player
	phase   phase
	turn    int
	clock   time.Duration
	phaseAt time.Duration // when the phase or turn began
	waitTil time.Duration // pause after a shot
	winner  int
	log     []string
	last    [2]cell // last shot at each board, for highlighting
	hasLast [2]bool
}

// New starts a match.
func New(seats []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng, winner: -1}
	for i := range w.p {
		w.p[i] = &player{info: seats[i], bot: seats[i].Bot, board: randomBoard(rng), cx: N / 2, cy: N / 2}
	}
	return w
}

func (w *World) say(s string) {
	w.log = append(w.log, s)
	if len(w.log) > 3 {
		w.log = w.log[len(w.log)-3:]
	}
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	p := w.p[seat]
	switch w.phase {
	case placing:
		switch key {
		case "r":
			if !p.ready {
				p.board = randomBoard(w.rng)
			}
		case "enter", " ":
			p.ready = true
		}
	case battle:
		switch key {
		case "up", "w", "k":
			p.cy = (p.cy + N - 1) % N
		case "down", "s", "j":
			p.cy = (p.cy + 1) % N
		case "left", "a", "h":
			p.cx = (p.cx + N - 1) % N
		case "right", "d", "l":
			p.cx = (p.cx + 1) % N
		case "enter", " ", "f":
			if w.turn == seat && w.clock >= w.waitTil {
				w.fire(seat, p.cx, p.cy)
			}
		}
	}
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) {
	w.p[seat].bot = true
	w.p[seat].ready = true
}

func colName(x int) string { return string(rune('A' + x)) }

// fire takes a shot for seat. Invalid shots (already shot cells) are
// ignored.
func (w *World) fire(seat, x, y int) {
	enemy := w.p[1-seat]
	res := enemy.board.shoot(x, y)
	if res == resInvalid {
		return
	}
	me := w.p[seat]
	me.shots++
	w.last[1-seat], w.hasLast[1-seat] = cell{x, y}, true
	at := colName(x) + strconv.Itoa(y+1)
	switch res {
	case resMiss:
		w.say(me.info.Name + " fires at " + at + ": miss")
		w.turn = 1 - seat
		w.phaseAt = w.clock
	case resHit:
		me.hits++
		w.say(me.info.Name + " fires at " + at + ": hit!")
		w.phaseAt = w.clock
	case resSunk:
		me.hits++
		w.say(me.info.Name + " fires at " + at + ": sunk!")
		w.phaseAt = w.clock
		if enemy.board.shipsLeft() == 0 {
			w.phase, w.winner = over, seat
		}
	}
	w.waitTil = w.clock + showShot
}

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	switch w.phase {
	case placing:
		for _, p := range w.p {
			if p.bot && w.clock >= 2*time.Second {
				p.ready = true
			}
		}
		if (w.p[0].ready && w.p[1].ready) || w.clock >= placeTime {
			w.phase = battle
			w.turn = w.rng.IntN(2)
			w.phaseAt = w.clock
			w.say(w.p[w.turn].info.Name + " shoots first")
		}
	case battle:
		if w.clock < w.waitTil {
			return
		}
		p := w.p[w.turn]
		timedOut := w.clock-w.phaseAt >= turnTime
		if (p.bot && w.clock-w.phaseAt >= botDelay) || timedOut {
			x, y := w.aim(w.turn)
			if timedOut && !p.bot {
				w.say(p.info.Name + " ran out of time")
			}
			p.cx, p.cy = x, y
			w.fire(w.turn, x, y)
		}
	}
}

// Over implements multi.World.
func (w *World) Over() bool { return w.phase == over }

// Standings implements multi.World.
func (w *World) Standings() []multi.Standing {
	win := w.winner
	if win < 0 {
		win = 0
		if w.p[1].board.afloat() > w.p[0].board.afloat() {
			win = 1
		}
	}
	return []multi.Standing{
		{Seat: win, Detail: w.accuracy(win)},
		{Seat: 1 - win, Detail: w.accuracy(1 - win)},
	}
}

func (w *World) accuracy(seat int) string {
	p := w.p[seat]
	if p.shots == 0 {
		return "no shots"
	}
	return strconv.Itoa(p.hits*100/p.shots) + "% hits"
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	me, enemy := w.p[seat], w.p[1-seat]
	return []multi.Stat{
		{Label: "YOUR SHIPS", Value: strconv.Itoa(me.board.shipsLeft())},
		{Label: "THEIRS", Value: strconv.Itoa(enemy.board.shipsLeft())},
	}
}
