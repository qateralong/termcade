// Package alien is a survival game: every player is a flying saucer
// circling an alien non-stop. Space reverses your orbit. The alien fires
// beams, sweeps, sector blasts and energy orbs; the last saucer flying wins.
package alien

import (
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/multi"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

const (
	W = multi.ArenaW
	H = multi.ArenaH

	seats = 6

	cx, cy = 30.0, 18.0 // the alien
	rx, ry = 25.0, 14.0 // the orbit

	orbitSpeed  = 1.25 // radians per second
	saucerHalf  = 0.11 // a saucer's half-width, in radians of orbit
	bumpGap     = 0.30 // saucers closer than this bounce off each other
	roundLength = 4 * time.Minute
)

// Game returns Alien.
func Game() games.Game {
	return multi.New(multi.Config{
		Info: games.Info{
			ID:          "alien",
			Name:        "Alien",
			Icon:        "◉",
			Tagline:     "Orbit the alien. Dodge its attacks. Outlast everyone.",
			Description: "Every player is a flying saucer circling the alien non-stop. Press space to reverse your orbit and dodge its beams, sweeps and orbs. Saucers bounce off each other. Last saucer flying wins.",
			Kind:        games.Realtime,
			Players:     "2–6",
			Controls:    []string{"space", "reverse orbit"},
			Accent:      []string{theme.Mint, theme.Lime, theme.Violet},
			Art: []string{
				"        ◇         ◇",
				"   ◇       ╭───╮      ◇",
				"      ⋯⋯⋯⋯ │◉ ◉│ ⋯⋯⋯",
				"   ◇       ╰─▽─╯      ◇",
			},
		},
		Seats: seats,
		NewWorld: func(s []multi.SeatInfo, rng *rand.Rand) multi.World {
			return New(s, rng)
		},
	})
}

type saucer struct {
	info    multi.SeatInfo
	bot     bool
	angle   float64
	dir     float64 // +1 or -1
	alive   bool
	outAt   time.Duration
	flipCD  time.Duration // short cooldown after a bounce
	reactAt map[*hazard]time.Duration
	nextBot time.Duration
}

type hazardKind int

const (
	beam hazardKind = iota
	sweep
	sector
	orb
)

// hazard is one attack. It is announced at start, deadly between fire and
// end, and aimed at angle(t).
type hazard struct {
	kind             hazardKind
	start, fire, end time.Duration
	from, to         float64 // aim at fire and at end (they differ for sweeps)
	half             float64 // angular half-width
}

func (h *hazard) angle(t time.Duration) float64 {
	if h.end <= h.fire || t <= h.fire {
		return h.from
	}
	if t >= h.end {
		return h.to
	}
	k := float64(t-h.fire) / float64(h.end-h.fire)
	return h.from + (h.to-h.from)*k
}

func (h *hazard) live(t time.Duration) bool { return t >= h.fire && t <= h.end }

// hits reports whether a saucer at angle a is caught at time t.
func (h *hazard) hits(a float64, t time.Duration) bool {
	return h.live(t) && math.Abs(wrap(a-h.angle(t))) < h.half+saucerHalf
}

func wrap(a float64) float64 { return math.Remainder(a, 2*math.Pi) }

// World is one match.
type World struct {
	rng     *rand.Rand
	saucers []*saucer
	hazards []*hazard
	clock   time.Duration
	nextAtk time.Duration
	booms   []boomFx
	stars   [][2]int
}

type boomFx struct {
	x, y float64
	at   time.Duration
}

// New starts a match.
func New(seatsInfo []multi.SeatInfo, rng *rand.Rand) *World {
	w := &World{rng: rng, nextAtk: 1500 * time.Millisecond}
	n := len(seatsInfo)
	for i, s := range seatsInfo {
		d := 1.0
		if i%2 == 1 {
			d = -1
		}
		w.saucers = append(w.saucers, &saucer{
			info: s, bot: s.Bot, alive: true, dir: d,
			angle:   float64(i) * 2 * math.Pi / float64(n),
			reactAt: map[*hazard]time.Duration{},
		})
	}
	for i := 0; i < 40; i++ {
		w.stars = append(w.stars, [2]int{rng.IntN(W), rng.IntN(H)})
	}
	return w
}

// level grows every 20 seconds and speeds up the alien.
func (w *World) level() float64 { return w.clock.Seconds() / 20 }

func (w *World) telegraph() time.Duration {
	return time.Duration(math.Max(0.45, 1.0-0.08*w.level()) * float64(time.Second))
}

// Input implements multi.World.
func (w *World) Input(seat int, key string) {
	s := w.saucers[seat]
	switch key {
	case " ", "enter", "left", "right", "up", "down", "a", "d", "f":
		if s.alive {
			s.dir = -s.dir
		}
	}
}

// SetBot implements multi.World.
func (w *World) SetBot(seat int) { w.saucers[seat].bot = true }

// Step implements multi.World.
func (w *World) Step(dt time.Duration) {
	w.clock += dt
	secs := dt.Seconds()

	if w.clock >= w.nextAtk {
		w.attack()
	}

	for _, s := range w.saucers {
		if !s.alive {
			continue
		}
		if s.bot {
			w.think(s)
		}
		s.angle = wrap(s.angle + s.dir*orbitSpeed*secs)
		if s.flipCD > 0 {
			s.flipCD -= dt
		}
	}
	w.bounce()

	for _, s := range w.saucers {
		if !s.alive {
			continue
		}
		for _, h := range w.hazards {
			if h.hits(s.angle, w.clock) {
				s.alive, s.outAt = false, w.clock
				x, y := pos(s.angle, 1)
				w.booms = append(w.booms, boomFx{x, y, w.clock})
				break
			}
		}
	}

	kept := w.hazards[:0]
	for _, h := range w.hazards {
		if w.clock <= h.end+150*time.Millisecond {
			kept = append(kept, h)
		}
	}
	w.hazards = kept
}

// bounce makes saucers that run into each other swap directions.
func (w *World) bounce() {
	for i, a := range w.saucers {
		for _, b := range w.saucers[i+1:] {
			if !a.alive || !b.alive || a.flipCD > 0 || b.flipCD > 0 || a.dir == b.dir {
				continue
			}
			gap := wrap(b.angle - a.angle)
			// Approaching each other: a moves toward b and b toward a.
			if math.Abs(gap) < bumpGap && math.Signbit(gap) != math.Signbit(a.dir) {
				continue
			}
			if math.Abs(gap) < bumpGap {
				a.dir, b.dir = -a.dir, -b.dir
				a.flipCD, b.flipCD = 250*time.Millisecond, 250*time.Millisecond
			}
		}
	}
}

// attack launches the alien's next move.
func (w *World) attack() {
	t := w.clock
	tg := w.telegraph()
	lv := w.level()
	target := w.randomTarget()
	switch k := w.rng.IntN(10); {
	case k < 4: // a few fixed beams
		n := 1 + w.rng.IntN(min(4, 2+int(lv)))
		base := target
		for i := 0; i < n; i++ {
			a := base + float64(i)*2*math.Pi/float64(n)
			w.hazards = append(w.hazards, &hazard{kind: beam, start: t, fire: t + tg, end: t + tg + 350*time.Millisecond, from: a, to: a, half: 0.16})
		}
		w.nextAtk = t + tg + w.pause()
	case k < 6: // a sweeping beam
		span := 1.4 + 0.1*lv
		if w.rng.IntN(2) == 0 {
			span = -span
		}
		dur := time.Duration((1.6 - math.Min(0.6, 0.06*lv)) * float64(time.Second))
		w.hazards = append(w.hazards, &hazard{kind: sweep, start: t, fire: t + tg, end: t + tg + dur, from: target - span/2, to: target + span/2, half: 0.1})
		w.nextAtk = t + tg + dur + w.pause()
	case k < 8: // a wide sector blast
		w.hazards = append(w.hazards, &hazard{kind: sector, start: t, fire: t + tg + 200*time.Millisecond, end: t + tg + 500*time.Millisecond, from: target, to: target, half: 0.85})
		w.nextAtk = t + tg + w.pause()
	default: // a volley of orbs flying out to the orbit
		n := 3 + w.rng.IntN(2+int(lv))
		travel := 1200 * time.Millisecond
		for i := 0; i < n; i++ {
			a := target + (w.rng.Float64()-0.5)*2.4
			launch := t + time.Duration(i)*220*time.Millisecond
			w.hazards = append(w.hazards, &hazard{kind: orb, start: launch, fire: launch + travel - 60*time.Millisecond, end: launch + travel + 60*time.Millisecond, from: a, to: a, half: 0.13})
		}
		w.nextAtk = t + travel + time.Duration(n)*220*time.Millisecond + w.pause()
	}
}

func (w *World) pause() time.Duration {
	return time.Duration(math.Max(0.35, 1.3-0.1*w.level()) * float64(time.Second))
}

// randomTarget aims at a random surviving saucer's current position.
func (w *World) randomTarget() float64 {
	var alive []*saucer
	for _, s := range w.saucers {
		if s.alive {
			alive = append(alive, s)
		}
	}
	if len(alive) == 0 {
		return 0
	}
	return alive[w.rng.IntN(len(alive))].angle
}

func (w *World) alive() int {
	n := 0
	for _, s := range w.saucers {
		if s.alive {
			n++
		}
	}
	return n
}

// Over implements multi.World.
func (w *World) Over() bool {
	return w.alive() <= 1 || w.clock >= roundLength
}

// Standings implements multi.World: survivors first, then by how long each
// saucer lasted.
func (w *World) Standings() []multi.Standing {
	idx := make([]int, len(w.saucers))
	for i := range idx {
		idx[i] = i
	}
	lasted := func(s *saucer) time.Duration {
		if s.alive {
			return w.clock + time.Hour
		}
		return s.outAt
	}
	sort.SliceStable(idx, func(a, b int) bool { return lasted(w.saucers[idx[a]]) > lasted(w.saucers[idx[b]]) })
	res := make([]multi.Standing, len(idx))
	for i, s := range idx {
		sc := w.saucers[s]
		d := "survived"
		if !sc.alive {
			d = solo.Duration(int(sc.outAt / time.Millisecond))
		}
		res[i] = multi.Standing{Seat: s, Detail: d}
	}
	return res
}

// Stats implements multi.World.
func (w *World) Stats(seat int) []multi.Stat {
	return []multi.Stat{
		{Label: "ALIVE", Value: strconv.Itoa(w.alive()) + "/" + strconv.Itoa(len(w.saucers))},
		{Label: "TIME", Value: solo.Duration(int(w.clock / time.Millisecond))},
	}
}

// pos is the screen position at angle a and a fraction k of the orbit.
func pos(a, k float64) (float64, float64) {
	return cx + rx*k*math.Cos(a), cy + ry*k*math.Sin(a)
}
