// Package chickenrun is a gravity-flipping race: chickens run to the right,
// flip between floor and ceiling to dodge obstacles and holes, and anyone
// left behind the edge of the screen is out.
package chickenrun

import (
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

const (
	W = multi.ArenaW // pixels on screen
	H = multi.ArenaH

	seats     = 4
	tile      = 2 // pixels per tile
	size      = 2 // chicken size in pixels
	baseSpeed = 17.0
	maxSpeed  = 27.0
	speedGain = 0.25  // pixels per second, per second
	gravity   = 140.0 // pixels per second squared
	maxFall   = 42.0
	leadRoom  = 40 // pixels between the left edge and the leader
)

// Game returns Chicken Run.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "chicken-run",
			Name:        "Chicken Run",
			Icon:        "▲",
			Tagline:     "One button flips gravity. Don't fall off the world.",
			Description: "Four chickens race through a random level. Flip your gravity between floor and ceiling to dodge obstacles and holes. The screen follows the leader: fall behind its edge and you're out.",
			Kind:        games.Realtime,
			Players:     "4",
			Controls:    []string{"space", "flip gravity", "↑↓", "flip up / down"},
			Accent:      []string{theme.Amber, "#FF9F43", theme.Pink},
			Art: []string{
				"▀▀▀▀▀▀▀▀▀▀▀▀  ▀▀▀▀▀▀▀▀▀▀▀▀▀",
				"      ▼        █        ▼",
				"  ▲       ▲    █   ▲",
				"▄▄▄▄▄▄▄▄   ▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng, rng.IntN(len(levels)))
		},
	})
}

type state int

const (
	running state = iota
	finished
	out
)

type chicken struct {
	info   multi.SeatInfo
	bot    bool
	x, y   float64 // top-left, in level pixels
	vy     float64
	grav   float64 // +1 falls down, -1 falls up
	state  state
	doneAt time.Duration // finish or knock-out time
	why    string
	// bot state
	thinkAt time.Duration
}

// World is one race.
type World struct {
	rng      *rand.Rand
	lv       *level
	chickens []*chicken
	camX     float64
	speed    float64
	clock    time.Duration
	order    []int // finishers, in order
	flash    []flash
}

type flash struct {
	x, y float64
	at   time.Duration
}

// New starts a race on level number n.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand, n int) *World {
	w := &World{rng: rng, lv: buildLevel(levels[n]), speed: baseSpeed}
	for i, s := range seatsInfo {
		c := &chicken{info: s, bot: s.Bot, grav: 1}
		c.x = float64(8 + i*3)
		c.y = float64((levelRows-2)*tile - size)
		if i%2 == 1 { // half start on the ceiling
			c.grav = -1
			c.y = float64(2 * tile)
		}
		w.chickens = append(w.chickens, c)
	}
	return w
}

func (w *World) solid(px, py int, t time.Duration) bool {
	if px < 0 {
		return true
	}
	return w.lv.at(px/tile, int(math.Floor(float64(py)/tile))) || w.lv.moverAt(px, py, t)
}

// blocked reports whether a chicken at (x, y) overlaps anything solid at
// time t.
func (w *World) blocked(x, y float64, t time.Duration) bool {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	for py := y0; py < y0+size; py++ {
		for px := x0; px < x0+size; px++ {
			if py >= 0 && py < H && w.solid(px, py, t) {
				return true
			}
		}
	}
	return false
}

// spiked reports whether a chicken at (x, y) touches spikes.
func (w *World) spiked(x, y float64) bool {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	for py := y0; py < y0+size; py++ {
		for px := x0; px < x0+size; px++ {
			if px >= 0 && w.lv.spikeAt(px/tile, int(math.Floor(float64(py)/tile))) {
				return true
			}
		}
	}
	return false
}

func (w *World) grounded(c *chicken) bool {
	return w.blocked(c.x, c.y+c.grav, w.clock)
}

func (w *World) flip(c *chicken, to float64) {
	if c.state != running || !w.grounded(c) {
		return
	}
	if to == 0 {
		to = -c.grav
	}
	if to != c.grav {
		c.grav = to
		c.vy = 0
	}
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	c := w.chickens[seat]
	switch key {
	case " ", "enter", "f":
		w.flip(c, 0)
	case "up", "w", "k":
		w.flip(c, -1)
	case "down", "s", "j":
		w.flip(c, 1)
	}
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.chickens[seat].bot = true }

// physics advances one chicken by secs seconds, ending at time t. It
// returns why the chicken was knocked out, or "" if it's fine.
func (w *World) physics(c *chicken, secs, speed float64, t time.Duration) string {
	// A crusher sliding onto you is the end.
	if w.blocked(c.x, c.y, t) {
		return "crushed"
	}
	// Run forward; walls stop you.
	nx := c.x + speed*secs
	if !w.blocked(nx, c.y, t) {
		c.x = nx
	} else {
		// Creep up to the wall.
		for step := 0.25; step <= speed*secs; step += 0.25 {
			if w.blocked(c.x+0.25, c.y, t) {
				break
			}
			c.x += 0.25
		}
	}
	// Fall.
	c.vy = math.Max(-maxFall, math.Min(maxFall, c.vy+gravity*c.grav*secs))
	ny := c.y + c.vy*secs
	if !w.blocked(c.x, ny, t) {
		c.y = ny
	} else {
		// Land flush against the surface.
		for i := 0; i < 20 && !w.blocked(c.x, c.y+c.grav*0.25, t); i++ {
			c.y += c.grav * 0.25
		}
		c.vy = 0
	}
	switch {
	case w.spiked(c.x, c.y):
		return "spiked"
	case c.y <= -size || c.y >= H:
		return "fell"
	}
	return ""
}

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	secs := dt.Seconds()
	w.speed = math.Min(maxSpeed, (baseSpeed+speedGain*w.clock.Seconds())*w.lv.spec.speed)

	for i, c := range w.chickens {
		if c.state != running {
			continue
		}
		if c.bot {
			w.think(c)
		}
		if why := w.physics(c, secs, w.speed, w.clock); why != "" {
			w.knockOut(i, c, why)
			continue
		}
		if c.x >= float64(w.lv.finish*tile) {
			c.state, c.doneAt = finished, w.clock
			w.order = append(w.order, i)
		}
	}

	// The camera follows the leader and never goes back.
	lead := w.camX
	for _, c := range w.chickens {
		if c.state == running {
			lead = math.Max(lead, c.x-leadRoom)
		}
	}
	w.camX = math.Max(w.camX, math.Min(lead, float64(w.lv.cols*tile-W)))
	for i, c := range w.chickens {
		if c.state == running && c.x+size < w.camX {
			w.knockOut(i, c, "left behind")
		}
	}
}

func (w *World) knockOut(i int, c *chicken, why string) {
	c.state, c.doneAt, c.why = out, w.clock, why
	w.flash = append(w.flash, flash{x: c.x, y: math.Max(0, math.Min(H-1, c.y)), at: w.clock})
}

func (w *World) runners() int {
	n := 0
	for _, c := range w.chickens {
		if c.state == running {
			n++
		}
	}
	return n
}

// Over implements multi.World: the race ends when someone crosses the line,
// or when at most one chicken is left running.
func (w *World) Over() bool {
	return len(w.order) > 0 || w.runners() <= 1
}

// Standings implements multi.World: finishers, then whoever is still
// running (furthest first), then the knocked out (last to go first).
func (w *World) Standings() []multi.Standing {
	var res []multi.Standing
	for _, i := range w.order {
		res = append(res, multi.Standing{Seat: i, Detail: "finished"})
	}
	var rest []int
	for i, c := range w.chickens {
		if c.state != finished {
			rest = append(rest, i)
		}
	}
	sort.SliceStable(rest, func(a, b int) bool {
		ca, cb := w.chickens[rest[a]], w.chickens[rest[b]]
		if (ca.state == running) != (cb.state == running) {
			return ca.state == running
		}
		if ca.state == running {
			return ca.x > cb.x
		}
		if ca.doneAt != cb.doneAt {
			return ca.doneAt > cb.doneAt
		}
		return ca.x > cb.x
	})
	for _, i := range rest {
		c := w.chickens[i]
		detail := w.progress(c) + " run"
		if c.state == out {
			detail = c.why
		}
		res = append(res, multi.Standing{Seat: i, Detail: detail})
	}
	return res
}

func (w *World) progress(c *chicken) string {
	p := int(100 * c.x / float64(w.lv.finish*tile))
	return strconv.Itoa(max(0, min(100, p))) + "%"
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	c := w.chickens[seat]
	return []multi.Stat{
		{Label: "LEVEL", Value: w.lv.spec.name},
		{Label: "DISTANCE", Value: w.progress(c)},
	}
}

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	cv := canvas.New(W, H)
	cam := int(w.camX)
	for y := 0; y < H; y++ {
		for sx := 0; sx < W; sx++ {
			px := cam + sx
			col, row := px/tile, y/tile
			if !w.lv.at(col, row) {
				// Checkered finish line.
				if col == w.lv.finish && (y/2+px)%2 == 0 {
					cv.Set(sx, y, "#FFFFFF")
				}
				continue
			}
			// Grass on surfaces facing open space, dirt inside.
			edge := !w.lv.at(col, row-1) || !w.lv.at(col, row+1)
			switch {
			case edge && (!w.lv.at(col, (y-1)/tile) || !w.lv.at(col, (y+1)/tile)):
				cv.Set(sx, y, "#4FB85C")
			case (px*7+y*3)%11 == 0:
				cv.Set(sx, y, "#8A6A4E")
			default:
				cv.Set(sx, y, "#6B4F3A")
			}
		}
	}

	// Spikes: pale tips on a dark red base, pointing away from the surface
	// they sit on (or both ways for blocks hanging in mid-air).
	for y := 0; y < H; y++ {
		for sx := 0; sx < W; sx++ {
			px := cam + sx
			col, row := px/tile, y/tile
			if !w.lv.spikeAt(col, row) {
				continue
			}
			tip := (px+y)%2 == 0
			switch {
			case tip:
				cv.Set(sx, y, "#E8E8F0")
			default:
				cv.Set(sx, y, "#B03040")
			}
		}
	}
	// Crushers.
	for _, m := range w.lv.movers {
		top := m.top(w.clock) * tile
		for y := 0; y < m.height*tile; y++ {
			for x := 0; x < m.width*tile; x++ {
				sx := m.col*tile + x - cam
				py := int(top) + y
				if sx < 0 || sx >= W || py < 0 || py >= H {
					continue
				}
				col := "#8A93A6"
				if (x+y)%2 == 0 {
					col = "#C5CCDA"
				}
				if y == 0 || y == m.height*tile-1 {
					col = "#FFC940" // hazard stripes on the ends
				}
				cv.Set(sx, py, col)
			}
		}
	}

	// Draw others first, you on top.
	order := make([]int, 0, len(w.chickens))
	for i := range w.chickens {
		if i != seat {
			order = append(order, i)
		}
	}
	order = append(order, seat)
	for _, i := range order {
		c := w.chickens[i]
		if c.state == out {
			continue
		}
		x, y := int(c.x)-cam, int(math.Floor(c.y))
		body, beak := c.info.Color, "#FFB020"
		comb := "#FF4F5E"
		// 2×2 sprite; the head faces right and flips with gravity.
		if c.grav > 0 {
			cv.Set(x, y, body)
			cv.Set(x+1, y, comb)
			cv.Set(x, y+1, body)
			cv.Set(x+1, y+1, beak)
		} else {
			cv.Set(x, y, body)
			cv.Set(x+1, y, beak)
			cv.Set(x, y+1, body)
			cv.Set(x+1, y+1, comb)
		}
		if i == seat && c.state == running {
			// A little marker over (or under) your chicken.
			my := y - 2
			if c.grav < 0 {
				my = y + 3
			}
			cv.Set(x, my, "#FFFFFF")
			cv.Set(x+1, my, "#FFFFFF")
		}
	}
	for _, f := range w.flash {
		age := w.clock - f.at
		if age > 600*time.Millisecond {
			continue
		}
		r := 1 + int(age/(150*time.Millisecond))
		x, y := int(f.x)-cam, int(f.y)
		for d := -r; d <= r; d += r {
			cv.Set(x+d, y, "#FFFFFF")
			cv.Set(x, y+d, "#FFFFFF")
		}
	}

	me := w.chickens[seat]
	if me.state == out {
		cv.TextCenter(H/4, " "+me.why+", watching ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.chickens))
	for _, st := range w.Standings() {
		c := w.chickens[st.Seat]
		v := w.progress(c)
		if c.state == finished {
			v = "done"
		}
		rows = append(rows, multi.SidebarRow{
			Name: c.info.Name, Color: c.info.Color, Value: v,
			You: st.Seat == seat, Out: c.state == out,
		})
	}
	bar := int(10 * w.camX / float64(max(1, w.lv.finish*tile-W)))
	progress := strings.Repeat("▰", min(10, max(0, bar))) + strings.Repeat("▱", 10-min(10, max(0, bar)))
	return multi.Arena(th, cv, strings.ToUpper(w.lv.spec.name), progress, rows, "* is you", "space: flip", "stay on screen!")
}
