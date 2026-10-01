// Package racing is top-down multiplayer car racing: three laps on one of
// several tracks, first across the line wins.
package racing

import (
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/games/solo"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

const (
	W = multi.ArenaW
	H = multi.ArenaH

	seats      = 4
	laps       = 3
	turnSteps  = 32 // headings in a full circle
	accel      = 28.0
	brake      = 45.0
	drag       = 10.0
	topSpeed   = 21.0
	grassSpeed = 8.0
	reverseMax = 6.0
	pedalGrant = 300 * time.Millisecond
	afterWin   = 20 * time.Second // time others get to finish
	raceLimit  = 4 * time.Minute
)

// Game returns Racing.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "racing",
			Name:        "Racing",
			Icon:        "▶",
			Tagline:     "Three laps. Several tracks. First across the line.",
			Description: "Top-down racing on the arrow keys. Stay on the tarmac (grass is slow), take the racing line and be first after three laps.",
			Kind:        games.Realtime,
			Players:     "4",
			Controls:    []string{"↑", "gas (hold)", "↓", "brake", "←→", "steer"},
			Accent:      []string{theme.Coral, theme.Amber, theme.Sky},
			Art: []string{
				"╭────────────────────────╮",
				"│  ▶ ▶      ╭──────╮  ▼  │",
				"│      ▶    ╰──────╯     │",
				"╰────────────────────────╯",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng, rng.IntN(len(tracks)))
		},
	})
}

type car struct {
	info    multi.SeatInfo
	bot     bool
	pos     vec
	heading int // 0..turnSteps-1
	speed   float64
	gas     time.Duration // throttle held until
	brakeT  time.Duration // brake held until
	dist    float64       // distance driven along the track, unwrapped
	along   float64       // last projected position, for unwrapping
	done    bool
	doneAt  time.Duration
	skill   float64 // bots: fraction of top speed they dare to use
}

func (c *car) angle() float64 { return float64(c.heading) * 2 * math.Pi / turnSteps }

// World is one race.
type World struct {
	rng    *rand.Rand
	tr     *track
	cars   []*car
	clock  time.Duration
	order  []int
	startS float64 // where the start/finish line is, along the track
}

// New starts a race on track number n.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand, n int) *World {
	w := &World{rng: rng, tr: buildTrack(tracks[n])}
	// The line sits in the middle of the first straight.
	w.startS = w.tr.cum[1] / 2
	for i, s := range seatsInfo {
		// A staggered grid behind the line.
		row, col := float64(i/2), float64(i%2)
		at := w.startS - 2 - row*3
		dir := w.tr.direction(at)
		p := w.tr.pointAt(at).add(unit(dir + math.Pi/2).mul((col - 0.5) * 3))
		heading := int(math.Round(dir/(2*math.Pi)*turnSteps)+turnSteps) % turnSteps
		c := &car{info: s, bot: s.Bot, pos: p, heading: heading, skill: 1}
		if s.Bot {
			c.skill = 0.74 + rng.Float64()*0.14
		}
		c.along, _ = w.tr.project(p)
		c.dist = w.tr.wrap(c.along - w.startS) // slightly negative: behind the line
		w.cars = append(w.cars, c)
	}
	return w
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	c := w.cars[seat]
	if c.done {
		return
	}
	switch key {
	case "up", "w", "k":
		c.gas, c.brakeT = w.clock+pedalGrant, 0
	case "down", "s", "j":
		c.brakeT, c.gas = w.clock+pedalGrant, 0
	case "left", "a", "h":
		c.heading = (c.heading + turnSteps - 1) % turnSteps
	case "right", "d", "l":
		c.heading = (c.heading + 1) % turnSteps
	}
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) {
	c := w.cars[seat]
	c.bot = true
	if c.skill >= 1 {
		c.skill = 0.9
	}
}

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	secs := dt.Seconds()
	for i, c := range w.cars {
		if c.done {
			// Coast to a stop after the flag.
			c.speed = math.Max(0, c.speed-brake*secs)
		} else if c.bot {
			w.drive(c)
		}
		w.move(c, secs)
		if !c.done && c.dist >= float64(laps)*w.tr.length {
			c.done, c.doneAt = true, w.clock
			w.order = append(w.order, i)
		}
	}
}

func (w *World) move(c *car, secs float64) {
	onGas := w.clock < c.gas
	onBrake := w.clock < c.brakeT
	limit := topSpeed
	if w.tr.surface(c.pos) == 0 {
		limit = grassSpeed
	}
	switch {
	case onGas:
		c.speed += accel * secs
	case onBrake && c.speed > 0:
		c.speed -= brake * secs
	case onBrake:
		c.speed = math.Max(-reverseMax, c.speed-accel*0.5*secs)
	default:
		if c.speed > 0 {
			c.speed = math.Max(0, c.speed-drag*secs)
		} else {
			c.speed = math.Min(0, c.speed+drag*secs)
		}
	}
	if c.speed > limit {
		// Grass bleeds speed quickly rather than stopping you dead.
		c.speed = math.Max(limit, c.speed-brake*1.5*secs)
	}

	next := c.pos.add(unit(c.angle()).mul(c.speed * secs))
	next.x = clampf(next.x, 0.5, W-1.5)
	next.y = clampf(next.y, 0.5, H-1.5)
	c.pos = next

	along, _ := w.tr.project(c.pos)
	c.dist += w.tr.wrap(along - c.along)
	c.along = along
}

// raceDone reports whether everyone is in, or the winner's grace period is
// over.
func (w *World) raceDone() bool {
	all := true
	for _, c := range w.cars {
		if !c.done {
			all = false
		}
	}
	if all || w.clock >= raceLimit {
		return true
	}
	if len(w.order) > 0 {
		return w.clock-w.cars[w.order[0]].doneAt >= afterWin
	}
	return false
}

// Over implements multi.World.
func (w *World) Over() bool { return w.raceDone() }

// Standings implements multi.World: finishers by time, then everyone else by
// distance covered.
func (w *World) Standings() []multi.Standing {
	var res []multi.Standing
	for _, i := range w.order {
		res = append(res, multi.Standing{Seat: i, Detail: solo.Duration(int(w.cars[i].doneAt / time.Millisecond))})
	}
	var rest []int
	for i, c := range w.cars {
		if !c.done {
			rest = append(rest, i)
		}
	}
	sort.SliceStable(rest, func(a, b int) bool { return w.cars[rest[a]].dist > w.cars[rest[b]].dist })
	for _, i := range rest {
		res = append(res, multi.Standing{Seat: i, Detail: "lap " + strconv.Itoa(w.lap(w.cars[i]))})
	}
	return res
}

func (w *World) lap(c *car) int {
	return min(laps, max(1, int(c.dist/w.tr.length)+1))
}

func (w *World) place(seat int) int {
	for i, st := range w.Standings() {
		if st.Seat == seat {
			return i
		}
	}
	return 0
}

var ordinals = []string{"1st", "2nd", "3rd", "4th"}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	c := w.cars[seat]
	lap := strconv.Itoa(w.lap(c)) + "/" + strconv.Itoa(laps)
	if c.done {
		lap = "done"
	}
	return []multi.Stat{
		{Label: "LAP", Value: lap},
		{Label: "POS", Value: ordinals[min(w.place(seat), len(ordinals)-1)]},
		{Label: "TIME", Value: solo.Duration(int(w.clock / time.Millisecond))},
	}
}

// View implements multi.World.
func (w *World) View(seat int, th *theme.Theme) string {
	cv := canvas.New(W, H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			switch w.tr.road[y][x] {
			case 1:
				cv.Set(x, y, "#3A3A48")
			case 2:
				if (x+y)%2 == 0 {
					cv.Set(x, y, "#E8E8F0")
				} else {
					cv.Set(x, y, theme.Coral)
				}
			default:
				if (x*5+y*3)%7 == 0 {
					cv.Set(x, y, "#2F7A3A")
				} else {
					cv.Set(x, y, "#23602D")
				}
			}
		}
	}
	// Checkered start/finish line across the road.
	start := w.tr.pointAt(w.startS)
	side := unit(w.tr.direction(w.startS) + math.Pi/2)
	for k := -w.tr.spec.width / 2; k <= w.tr.spec.width/2; k += 0.5 {
		p := start.add(side.mul(k))
		x, y := int(p.x), int(p.y)
		if w.tr.surface(p) == 1 {
			if (x+y)%2 == 0 {
				cv.Set(x, y, "#FFFFFF")
			} else {
				cv.Set(x, y, "#111111")
			}
		}
	}

	// Cars: a rear pixel in the car's color and a lighter nose pixel.
	order := make([]int, 0, len(w.cars))
	for i := range w.cars {
		if i != seat {
			order = append(order, i)
		}
	}
	order = append(order, seat)
	for _, i := range order {
		c := w.cars[i]
		nose := c.pos.add(unit(c.angle()).mul(0.9))
		tail := c.pos.sub(unit(c.angle()).mul(0.6))
		cv.Set(int(tail.x), int(tail.y), c.info.Color)
		noseCol := theme.Mix(c.info.Color, "#FFFFFF", 0.6)
		if i == seat {
			noseCol = "#FFFFFF"
		}
		cv.Set(int(nose.x), int(nose.y), noseCol)
	}

	me := w.cars[seat]
	if me.done {
		cv.TextCenter(H/2/2, " finished "+ordinals[min(w.place(seat), 3)]+"! ", "#1A1325", theme.Amber)
	} else if len(w.order) > 0 {
		left := int((afterWin - (w.clock - w.cars[w.order[0]].doneAt) + time.Second - 1) / time.Second)
		cv.TextCenter(H/2/2, " flag in "+strconv.Itoa(left)+"s ", "#FFFFFF", theme.Coral)
	}

	rows := make([]multi.SidebarRow, 0, len(w.cars))
	for _, st := range w.Standings() {
		c := w.cars[st.Seat]
		v := "L" + strconv.Itoa(w.lap(c))
		if c.done {
			v = "✓"
		}
		rows = append(rows, multi.SidebarRow{Name: c.info.Name, Color: c.info.Color, Value: v, You: st.Seat == seat})
	}
	speed := int(math.Abs(me.speed) / topSpeed * 100)
	gauge := strings.Repeat("▰", min(10, speed/10)) + strings.Repeat("▱", 10-min(10, speed/10))
	return multi.Arena(th, cv, strings.ToUpper(w.tr.spec.name), gauge, rows, "* is you", "grass is slow")
}
