// Package tanks is a top-down tank battle in the style of the NES classic:
// destructible brick, solid steel, water and bushes, three lives each, last
// tank rolling wins.
package tanks

import (
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
	W = multi.ArenaW // pixels
	H = multi.ArenaH

	seats       = 4
	tankSize    = 4
	startLives  = 3
	tankSpeed   = 14.0 // pixels per second
	shellSpeed  = 48.0
	driveGrant  = 280 * time.Millisecond // how long one key press drives
	reloadTime  = 450 * time.Millisecond
	respawnTime = 2 * time.Second
	shieldTime  = 2 * time.Second
	roundLength = 3 * time.Minute
	boomTime    = 450 * time.Millisecond
)

// Game returns Tanks.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "tanks",
			Name:        "Tanks",
			Icon:        "▣",
			Tagline:     "Brick walls, steel walls and a lot of shells.",
			Description: "Top-down tank battles in the style of the NES classic. Blast through bricks, hide in the bushes, and be the last tank rolling. Three lives each.",
			Kind:        games.Realtime,
			Players:     "2–4",
			Controls:    []string{"←↑→↓", "drive (hold)", "space", "fire"},
			Accent:      []string{theme.Amber, theme.Coral, theme.Lime},
			Art: []string{
				" ▓▓▓▓  ░░░░      ▓▓▓▓",
				" ▓▓▓▓  ░░░░  ▲   ▓▓▓▓",
				"       ░░░░  █",
				"   ▀█▀     ·  ·  ·  ▄█▄",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng, rng.IntN(len(quadrants)))
		},
	})
}

type dir int

const (
	up dir = iota
	right
	down
	left
)

func (d dir) vec() (int, int) {
	switch d {
	case up:
		return 0, -1
	case right:
		return 1, 0
	case down:
		return 0, 1
	}
	return -1, 0
}

// Pixel materials.
const (
	empty byte = iota
	brick
	steel
	water
	bush
)

type tank struct {
	info   multi.SeatInfo
	bot    bool
	x, y   int // top-left pixel
	dir    dir
	acc    float64
	drive  time.Duration // keep driving until this time
	fire   bool          // fire on the next step
	lives  int
	kills  int
	alive  bool
	outAt  time.Duration // when it was destroyed
	since  time.Duration // when it (re)spawned, for the shield
	reload time.Duration
	spawn  [2]int
	// bot state
	thinkAt time.Duration
	stuck   int
}

type shell struct {
	x, y  int
	dir   dir
	owner int
	acc   float64
	dead  bool
}

type boom struct {
	x, y int
	at   time.Duration
}

// World is one battle.
type World struct {
	rng     *rand.Rand
	mapName string
	pix     [H][W]byte
	tanks   []*tank
	shells  []*shell
	booms   []boom
	clock   time.Duration
	elim    []int // seats in the order they were knocked out
}

// New starts a battle on map number m.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand, m int) *World {
	w := &World{rng: rng, mapName: quadrants[m].name}
	tiles := buildMap(quadrants[m].rows)
	for ty := 0; ty < TilesH; ty++ {
		for tx := 0; tx < TilesW; tx++ {
			var mat byte
			switch tiles[ty][tx] {
			case '#':
				mat = brick
			case '@':
				mat = steel
			case '~':
				mat = water
			case '%':
				mat = bush
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					w.pix[ty*2+dy][tx*2+dx] = mat
				}
			}
		}
	}
	for i, s := range seatsInfo {
		t := &tank{info: s, bot: s.Bot, lives: startLives, spawn: spawns[i%len(spawns)]}
		w.tanks = append(w.tanks, t)
		w.respawn(t)
	}
	return w
}

func (w *World) respawn(t *tank) {
	t.x, t.y = t.spawn[0]*2, t.spawn[1]*2
	t.dir = down
	if t.spawn[1] > TilesH/2 {
		t.dir = up
	}
	t.alive, t.since, t.acc, t.drive = true, w.clock, 0, 0
}

func (w *World) shielded(t *tank) bool { return w.clock-t.since < shieldTime }

// solidAt reports whether a tank can't enter pixel (x, y).
func (w *World) solidAt(x, y int) bool {
	if x < 0 || y < 0 || x >= W || y >= H {
		return true
	}
	m := w.pix[y][x]
	return m == brick || m == steel || m == water
}

// fits reports whether tank t may occupy (x, y).
func (w *World) fits(t *tank, x, y int) bool {
	for dy := 0; dy < tankSize; dy++ {
		for dx := 0; dx < tankSize; dx++ {
			if w.solidAt(x+dx, y+dy) {
				return false
			}
		}
	}
	for _, o := range w.tanks {
		if o != t && o.alive && x < o.x+tankSize && o.x < x+tankSize && y < o.y+tankSize && o.y < y+tankSize {
			return false
		}
	}
	return true
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	t := w.tanks[seat]
	switch key {
	case "up", "w", "k":
		w.steer(t, up)
	case "right", "d", "l":
		w.steer(t, right)
	case "down", "s", "j":
		w.steer(t, down)
	case "left", "a", "h":
		w.steer(t, left)
	case " ", "enter", "f":
		t.fire = true
	}
}

// steer turns the tank and keeps it driving for a moment. Turning onto the
// other axis snaps the tank to the half-tile grid so it slides into
// corridors easily.
func (w *World) steer(t *tank, d dir) {
	if !t.alive {
		return
	}
	if (d == up || d == down) != (t.dir == up || t.dir == down) {
		nx, ny := t.x, t.y
		if d == up || d == down {
			nx = (t.x + 1) / 2 * 2
		} else {
			ny = (t.y + 1) / 2 * 2
		}
		if w.fits(t, nx, ny) {
			t.x, t.y = nx, ny
		}
	}
	t.dir = d
	t.drive = w.clock + driveGrant
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.tanks[seat].bot = true }

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	secs := dt.Seconds()

	for i, t := range w.tanks {
		if !t.alive {
			if t.lives > 0 && w.clock-t.outAt >= respawnTime {
				w.respawn(t)
			}
			continue
		}
		if t.bot {
			w.think(i, t)
		}
		if t.reload > 0 {
			t.reload -= dt
		}
		if t.fire {
			t.fire = false
			if t.reload <= 0 && w.shellsOf(i) == 0 {
				w.shoot(i, t)
			}
		}
		if w.clock < t.drive {
			t.acc += tankSpeed * secs
			for t.acc >= 1 {
				t.acc--
				dx, dy := t.dir.vec()
				if w.fits(t, t.x+dx, t.y+dy) {
					t.x += dx
					t.y += dy
					t.stuck = 0
				} else {
					t.acc = 0
					t.stuck++
				}
			}
		} else {
			t.acc = 0
		}
	}

	for _, s := range w.shells {
		s.acc += shellSpeed * secs
		for s.acc >= 1 && !s.dead {
			s.acc--
			w.moveShell(s)
		}
	}
	live := w.shells[:0]
	for _, s := range w.shells {
		if !s.dead {
			live = append(live, s)
		}
	}
	w.shells = live

	kept := w.booms[:0]
	for _, b := range w.booms {
		if w.clock-b.at < boomTime {
			kept = append(kept, b)
		}
	}
	w.booms = kept
}

func (w *World) shellsOf(i int) int {
	n := 0
	for _, s := range w.shells {
		if s.owner == i && !s.dead {
			n++
		}
	}
	return n
}

func (w *World) shoot(i int, t *tank) {
	t.reload = reloadTime
	// Shells leave from the middle of the barrel side.
	x, y := t.x+1, t.y+1
	switch t.dir {
	case up:
		y = t.y - 1
	case down:
		y = t.y + tankSize
	case left:
		x = t.x - 1
	case right:
		x = t.x + tankSize
	}
	if t.dir == up || t.dir == down {
		x = t.x + 1 + w.rng.IntN(2)
	} else {
		y = t.y + 1 + w.rng.IntN(2)
	}
	s := &shell{x: x, y: y, dir: t.dir, owner: i}
	w.shells = append(w.shells, s)
	w.hit(s) // it may be fired point-blank into something
}

func (w *World) moveShell(s *shell) {
	dx, dy := s.dir.vec()
	s.x += dx
	s.y += dy
	w.hit(s)
}

// hit resolves what a shell at its current position runs into.
func (w *World) hit(s *shell) {
	if s.x < 0 || s.y < 0 || s.x >= W || s.y >= H {
		s.dead = true
		return
	}
	switch w.pix[s.y][s.x] {
	case steel:
		s.dead = true
		w.booms = append(w.booms, boom{s.x, s.y, w.clock})
		return
	case brick:
		// Knock out a two-pixel-wide chunk across the shell's path.
		w.pix[s.y][s.x] = empty
		if s.dir == up || s.dir == down {
			w.clearBrick(s.x^1, s.y)
		} else {
			w.clearBrick(s.x, s.y^1)
		}
		s.dead = true
		w.booms = append(w.booms, boom{s.x, s.y, w.clock})
		return
	}
	for i, t := range w.tanks {
		if !t.alive || i == s.owner {
			continue
		}
		if s.x >= t.x && s.x < t.x+tankSize && s.y >= t.y && s.y < t.y+tankSize {
			s.dead = true
			if w.shielded(t) {
				return
			}
			w.destroy(i, t, s.owner)
			return
		}
	}
	for _, o := range w.shells {
		if o != s && !o.dead && o.x == s.x && o.y == s.y {
			o.dead, s.dead = true, true
			return
		}
	}
}

func (w *World) clearBrick(x, y int) {
	if x >= 0 && y >= 0 && x < W && y < H && w.pix[y][x] == brick {
		w.pix[y][x] = empty
	}
}

func (w *World) destroy(i int, t *tank, by int) {
	t.alive = false
	t.outAt = w.clock
	t.lives--
	if by >= 0 && by != i {
		w.tanks[by].kills++
	}
	w.booms = append(w.booms, boom{t.x + 1, t.y + 1, w.clock})
	if t.lives <= 0 {
		w.elim = append(w.elim, i)
	}
}

func (w *World) aliveCount() int {
	n := 0
	for _, t := range w.tanks {
		if t.lives > 0 {
			n++
		}
	}
	return n
}

// Over implements multi.World.
func (w *World) Over() bool { return w.aliveCount() <= 1 || w.clock >= roundLength }

// Standings implements multi.World: survivors by lives then kills, then the
// knocked-out in reverse order.
func (w *World) Standings() []multi.Standing {
	var alive []int
	for i, t := range w.tanks {
		if t.lives > 0 {
			alive = append(alive, i)
		}
	}
	sort.SliceStable(alive, func(a, b int) bool {
		ta, tb := w.tanks[alive[a]], w.tanks[alive[b]]
		if ta.lives != tb.lives {
			return ta.lives > tb.lives
		}
		return ta.kills > tb.kills
	})
	order := alive
	for i := len(w.elim) - 1; i >= 0; i-- {
		order = append(order, w.elim[i])
	}
	out := make([]multi.Standing, len(order))
	for i, s := range order {
		k := w.tanks[s].kills
		detail := strconv.Itoa(k) + " kills"
		if k == 1 {
			detail = "1 kill"
		}
		out[i] = multi.Standing{Seat: s, Detail: detail}
	}
	return out
}

func hearts(n int) string {
	if n <= 0 {
		return "✗"
	}
	return strings.Repeat("♥", n)
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	t := w.tanks[seat]
	return []multi.Stat{
		{Label: "LIVES", Value: hearts(t.lives)},
		{Label: "KILLS", Value: strconv.Itoa(t.kills)},
		{Label: "TIME", Value: multi.Clock(int((roundLength - w.clock + time.Second - 1) / time.Second))},
	}
}

// ---------------------------------------------------------------------------
// Drawing

// Sprites are 4×4: T track, C body, B barrel, . body shade. Drawn facing up
// and rotated for the other directions.
var sprite = [4]string{
	"TBBT",
	"TCCT",
	"TC.T",
	"TCCT",
}

func spritePixel(d dir, x, y int) byte {
	switch d {
	case right: // rotate clockwise: (x,y) <- (y, 3-x)
		return sprite[3-x][y]
	case down:
		return sprite[3-y][3-x]
	case left:
		return sprite[x][3-y]
	}
	return sprite[y][x]
}

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	c := canvas.New(W, H)
	waterPhase := int(w.clock/(400*time.Millisecond)) % 2
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			switch w.pix[y][x] {
			case brick:
				if (y%2 == 0) != ((x/2+y/2)%2 == 0) {
					c.Set(x, y, "#C0583A")
				} else {
					c.Set(x, y, "#8E3B26")
				}
			case steel:
				if (x+y)%2 == 0 {
					c.Set(x, y, "#D5DCE8")
				} else {
					c.Set(x, y, "#8A93A6")
				}
			case water:
				if (x+y+waterPhase)%3 == 0 {
					c.Set(x, y, "#5A8BFF")
				} else {
					c.Set(x, y, "#2F5FE0")
				}
			}
		}
	}

	for i, t := range w.tanks {
		if !t.alive {
			continue
		}
		if w.shielded(t) && (w.clock/(120*time.Millisecond))%2 == 0 {
			continue // blink while shielded
		}
		body := t.info.Color
		track := theme.Mix(body, "#000000", 0.45)
		shade := theme.Mix(body, "#000000", 0.2)
		barrel := "#F2F2F2"
		if i == seat {
			barrel = "#FFFFFF"
		}
		for y := 0; y < tankSize; y++ {
			for x := 0; x < tankSize; x++ {
				var col string
				switch spritePixel(t.dir, x, y) {
				case 'T':
					col = track
				case 'C':
					col = body
				case 'B':
					col = barrel
				case '.':
					col = shade
				}
				c.Set(t.x+x, t.y+y, col)
			}
		}
	}

	for _, s := range w.shells {
		c.Set(s.x, s.y, "#FFE066")
	}
	for _, b := range w.booms {
		age := float64(w.clock-b.at) / float64(boomTime)
		r := 1 + int(age*2)
		col := theme.Mix("#FFE066", theme.Coral, age)
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r && (dx+dy+int(age*4))%2 == 0 {
					c.Set(b.x+dx, b.y+dy, col)
				}
			}
		}
	}

	// Bushes cover everything underneath.
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if w.pix[y][x] == bush {
				if (x*3+y)%4 == 0 {
					c.Set(x, y, "#4FB85C")
				} else {
					c.Set(x, y, "#2E7D3A")
				}
			}
		}
	}

	me := w.tanks[seat]
	switch {
	case me.lives <= 0:
		c.TextCenter(H/4, " knocked out, watching ", "#FFFFFF", theme.Coral)
	case !me.alive:
		left := (respawnTime - (w.clock - me.outAt) + time.Second - 1) / time.Second
		c.TextCenter(H/4, " respawning in "+strconv.Itoa(int(left))+" ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.tanks))
	for _, st := range w.Standings() {
		t := w.tanks[st.Seat]
		rows = append(rows, multi.SidebarRow{
			Name: t.info.Name, Color: t.info.Color, Value: hearts(t.lives),
			You: st.Seat == seat, Out: t.lives <= 0,
		})
	}
	secs := int((roundLength - w.clock + time.Second - 1) / time.Second)
	return multi.Arena(th, c, strings.ToUpper(w.mapName), multi.Clock(secs), rows, "* is you", "last tank wins")
}
