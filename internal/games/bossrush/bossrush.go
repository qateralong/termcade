// Package bossrush is a bullet-hell boss rush: your heart dodges attacks in
// a box, and on your turn you strike with a timing bar. Five bosses, back to
// back.
package bossrush

import (
	"math"
	"math/rand/v2"
	"strconv"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

const (
	maxHP      = 20
	startItems = 3
	healAmount = 10
	invulnTime = 800 * time.Millisecond
	attackTime = 7 * time.Second
	barTime    = 1200 * time.Millisecond // one sweep of the timing bar
	maxHit     = 20                      // damage of a perfect strike
	heartSpeed = 16.0                    // cells per second
	holdGrant  = 160 * time.Millisecond
	jumpSpeed  = 9.0
	fall       = 22.0
	introTime  = 2500 * time.Millisecond
)

// Game returns Boss Rush.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "bossrush",
			Name:        "Boss Rush",
			Icon:        "♥",
			Tagline:     "Five bosses, one heart. Dodge, strike, survive.",
			Description: "Your heart dodges each boss's attacks inside the box: bone walls, fire, lasers, rings, and gravity that makes you jump. On your turn, stop the timing bar near the middle to hit hard. Beat all five in a row.",
			Kind:        games.Realtime,
			Players:     "1",
			Controls:    []string{"←↑→↓", "move the heart", "↑", "jump (blue heart)", "enter", "choose / strike", "←→", "menu"},
			Accent:      []string{theme.Coral, theme.Pink, theme.Violet},
			Art: []string{
				"     ▄█████▄",
				"    █  ◉ ◉  █",
				"  ┌──────────────┐",
				"  │ ┃     ♥   ┃  │",
				"  └──────────────┘",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 25 * time.Millisecond,
	})
}

type phase int

const (
	intro  phase = iota // the boss appears
	menu                // choose FIGHT or HEAL
	strike              // the timing bar
	dodge               // the boss attacks
	beaten              // the boss is down
)

// Engine is one run through the bosses.
type Engine struct {
	th  *theme.Theme
	rng *rand.Rand

	stage    int // which boss
	bossHP   int
	hp       int
	items    int
	phase    phase
	phaseAt  time.Duration
	clock    time.Duration
	menuSel  int
	barPos   float64 // 0..1 while striking
	lastHit  int     // damage of the last strike, for display
	lastNote string
	line     string

	hx, hy  float64 // the heart
	vx, vy  float64
	holdX   time.Duration
	holdY   time.Duration
	blue    bool // gravity mode
	onFloor bool
	hurtAt  time.Duration

	att     *attack
	bullets []*bullet
	beams   []*beam

	score int
	state solo.State
}

// New starts a run.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, hp: maxHP, items: startItems, hurtAt: -time.Hour}
	e.meet(0)
	return e
}

func (e *Engine) boss() *boss { return &bosses[e.stage] }

func (e *Engine) meet(n int) {
	e.stage = n
	e.bossHP = e.boss().hp
	e.setPhase(intro)
	e.line = e.boss().name + " blocks the way!"
}

func (e *Engine) setPhase(p phase) {
	e.phase, e.phaseAt = p, e.clock
}

func (e *Engine) since() time.Duration { return e.clock - e.phaseAt }

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	switch e.phase {
	case intro, beaten:
		if k == "enter" || k == " " || k == "z" {
			e.advance()
		}
	case menu:
		switch k {
		case "left", "a", "h", "right", "d", "l", "tab":
			e.menuSel = 1 - e.menuSel
		case "enter", " ", "z":
			e.choose()
		}
	case strike:
		if k == "enter" || k == " " || k == "z" {
			e.resolveStrike()
		}
	case dodge:
		e.steer(k)
	}
}

// advance leaves the intro or a beaten boss behind.
func (e *Engine) advance() {
	switch e.phase {
	case intro:
		e.setPhase(menu)
		e.line = e.boss().lines[0]
	case beaten:
		if e.stage+1 >= len(bosses) {
			e.score += e.hp * 100
			e.state = solo.Won
			return
		}
		e.hp = min(maxHP, e.hp+maxHP/2)
		e.items++
		e.meet(e.stage + 1)
	}
}

func (e *Engine) choose() {
	if e.menuSel == 1 {
		if e.items == 0 {
			e.line = "You're out of healing."
			return
		}
		e.items--
		gained := min(maxHP, e.hp+healAmount) - e.hp
		e.hp += gained
		e.line = "You recovered " + strconv.Itoa(gained) + " HP."
		e.startAttack()
		return
	}
	e.barPos = 0
	e.setPhase(strike)
}

func (e *Engine) resolveStrike() {
	// Damage falls off with distance from the middle of the bar.
	off := math.Abs(e.barPos - 0.5)
	acc := math.Max(0, 1-off*2.2)
	dmg := int(math.Round(maxHit * acc))
	switch {
	case off < 0.03:
		dmg = maxHit * 3 / 2
		e.lastNote = "CRITICAL!"
	case dmg == 0:
		e.lastNote = "MISS"
	default:
		e.lastNote = ""
	}
	e.lastHit = dmg
	e.bossHP = max(0, e.bossHP-dmg)
	e.score += dmg * 10
	if e.bossHP == 0 {
		e.score += 1000 * (e.stage + 1)
		e.setPhase(beaten)
		e.line = e.boss().name + " is defeated!"
		return
	}
	e.startAttack()
}

func (e *Engine) startAttack() {
	b := e.boss()
	kind := b.attacks[e.rng.IntN(len(b.attacks))]
	e.att = &attack{kind: kind, duration: attackTime}
	e.bullets, e.beams = nil, nil
	e.hx, e.hy = BoxW/2, BoxH/2
	e.vx, e.vy = 0, 0
	e.blue = kind == boneHop
	if e.blue {
		e.hy = BoxH - 1
	}
	e.setPhase(dodge)
}

// steer moves the heart. Each press moves a cell right away and keeps the
// heart moving for a moment, so holding a key glides smoothly.
func (e *Engine) steer(k string) {
	switch k {
	case "left", "a", "h":
		e.vx, e.holdX = -heartSpeed, e.clock+holdGrant
		e.hx = math.Max(0, e.hx-1)
	case "right", "d", "l":
		e.vx, e.holdX = heartSpeed, e.clock+holdGrant
		e.hx = math.Min(BoxW-1, e.hx+1)
	case "up", "w", "k":
		if e.blue {
			if e.onFloor {
				e.vy = -jumpSpeed
				e.onFloor = false
			}
			return
		}
		e.vy, e.holdY = -heartSpeed/2, e.clock+holdGrant
		e.hy = math.Max(0, e.hy-1)
	case "down", "s", "j":
		if e.blue {
			return
		}
		e.vy, e.holdY = heartSpeed/2, e.clock+holdGrant
		e.hy = math.Min(BoxH-1, e.hy+1)
	}
}

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	if e.state != solo.Playing {
		return
	}
	e.clock += dt
	secs := dt.Seconds()
	switch e.phase {
	case intro, beaten:
		if e.since() >= introTime {
			e.advance()
		}
	case strike:
		e.barPos = math.Mod(e.since().Seconds()/barTime.Seconds(), 1)
		if e.since() >= 2*barTime { // too slow: a weak swing
			e.barPos = 0.95
			e.resolveStrike()
		}
	case dodge:
		e.updateDodge(dt, secs)
	}
}

func (e *Engine) updateDodge(dt time.Duration, secs float64) {
	a := e.att
	a.elapsed += dt
	sp := e.boss().speed
	if a.elapsed < a.duration-time.Second {
		for a.elapsed >= a.next {
			e.spawn(a, sp)
		}
	}

	// Move the heart.
	if e.clock < e.holdX {
		e.hx += e.vx * secs
	}
	if e.blue {
		e.vy += fall * secs
		e.hy += e.vy * secs
		if e.hy >= BoxH-1 {
			e.hy, e.vy, e.onFloor = BoxH-1, 0, true
		}
	} else if e.clock < e.holdY {
		e.hy += e.vy * secs
	}
	e.hx = math.Max(0, math.Min(BoxW-1, e.hx))
	e.hy = math.Max(0, math.Min(BoxH-1, e.hy))

	// Move the bullets.
	kept := e.bullets[:0]
	for _, b := range e.bullets {
		b.x += b.vx * secs
		b.y += b.vy * secs
		if !b.gone() {
			kept = append(kept, b)
		}
	}
	e.bullets = kept
	live := e.beams[:0]
	for _, b := range e.beams {
		if a.elapsed < b.endAt {
			live = append(live, b)
		}
	}
	e.beams = live

	// Getting hit.
	x, y := int(math.Round(e.hx)), int(math.Round(e.hy))
	if e.clock-e.hurtAt >= invulnTime && e.touching(x, y) {
		e.hp -= e.boss().damage
		e.hurtAt = e.clock
		if e.hp <= 0 {
			e.hp = 0
			e.state = solo.Lost
			return
		}
	}

	if a.elapsed >= a.duration {
		e.bullets, e.beams = nil, nil
		e.setPhase(menu)
		lines := e.boss().lines
		e.line = lines[e.rng.IntN(len(lines))]
	}
}

func (e *Engine) touching(x, y int) bool {
	for _, b := range e.bullets {
		if b.hits(x, y) {
			return true
		}
	}
	for _, b := range e.beams {
		if e.att.elapsed >= b.fireAt && ((b.horiz && b.pos == y) || (!b.horiz && b.pos == x)) {
			return true
		}
	}
	return false
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.score }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{
		{Label: "BOSS", Value: strconv.Itoa(e.stage+1) + "/" + strconv.Itoa(len(bosses))},
		{Label: "HP", Value: strconv.Itoa(e.hp)},
	}
}
