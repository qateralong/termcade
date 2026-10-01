// Package invaders is the alien-shooting classic: a marching fleet, crumbling
// shields, the odd mystery ship, and wave after wave.
package invaders

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/canvas"
	"termcade/internal/ui/theme"
)

const (
	W = 60
	H = 36

	cols, rows   = 8, 5
	alienW       = 3
	alienH       = 2
	spacingX     = 6
	spacingY     = 4
	playerW      = 5
	playerY      = H - 3
	playerStep   = 2
	startLives   = 3
	shieldY      = H - 9
	bulletSpeed  = 40.0 // pixels per second
	bombSpeed    = 16.0
	ufoSpeed     = 10.0
	respawnPause = 2 * time.Second
)

var rowPoints = []int{30, 20, 20, 10, 10}
var rowColors = []string{theme.Pink, theme.Violet, theme.Violet, theme.Cyan, theme.Cyan}

// Two animation frames per alien type, 3×2 pixels each.
var alienFrames = [3][2][alienH]string{
	{{".#.", "#.#"}, {".#.", ".#."}}, // squid (top row)
	{{"#.#", "###"}, {"###", "#.#"}}, // crab
	{{"###", "#.#"}, {"###", ".#."}}, // octopus
}

func alienKind(row int) int {
	switch {
	case row == 0:
		return 0
	case row <= 2:
		return 1
	}
	return 2
}

// Game returns Space Invaders.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "invaders",
			Name:        "Space Invaders",
			Icon:        "▲",
			Tagline:     "They march, they drop, they speed up. Hold the line.",
			Description: "Shoot down the marching fleet before it lands. The fewer invaders are left, the faster they move. Hide behind the shields while they last, and tag the mystery ship for bonus points.",
			Kind:        games.Realtime,
			Players:     "1",
			Controls:    []string{"←→", "move (hold)", "space", "fire"},
			Accent:      []string{theme.Lime, theme.Cyan, theme.Violet},
			Art: []string{
				"  ▚▞  ▚▞  ▚▞  ▚▞  ▚▞",
				"  ▜▛  ▜▛  ▜▛  ▜▛  ▜▛",
				"",
				"  ▄▄▄      ▄▄▄    ▲",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 25 * time.Millisecond,
	})
}

type shot struct {
	x  int
	y  float64
	up bool
}

// Engine is one game of Space Invaders.
type Engine struct {
	th  *theme.Theme
	rng *rand.Rand

	alive   [rows][cols]bool
	count   int
	ox, oy  int // fleet offset
	dir     int
	march   time.Duration
	frame   int
	bombCD  time.Duration
	shields map[[2]int]bool

	player int // left edge
	bullet *shot
	bombs  []*shot
	ufo    float64 // x, or -100 when absent
	ufoDir float64
	ufoCD  time.Duration
	booms  []boom
	clock  time.Duration
	downAt time.Duration // when the player was hit

	score, lives, wave int
	state              solo.State
}

type boom struct {
	x, y int
	at   time.Duration
	big  bool
}

// New starts a game.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, lives: startLives, wave: 1, downAt: -time.Hour}
	e.player = (W - playerW) / 2
	e.buildShields()
	e.startWave()
	return e
}

func (e *Engine) buildShields() {
	e.shields = map[[2]int]bool{}
	shape := []string{".####.", "######", "##..##"}
	for i := 0; i < 4; i++ {
		x0 := 5 + i*14
		for y, row := range shape {
			for x, ch := range row {
				if ch == '#' {
					e.shields[[2]int{x0 + x, shieldY + y}] = true
				}
			}
		}
	}
}

func (e *Engine) startWave() {
	for r := range e.alive {
		for c := range e.alive[r] {
			e.alive[r][c] = true
		}
	}
	e.count = rows * cols
	e.ox = 4
	e.oy = 3 + min(e.wave-1, 4) // later waves start lower
	e.dir = 1
	e.bombs = nil
	e.bullet = nil
	e.bombCD = 1500 * time.Millisecond // a moment to get your bearings
	e.ufo, e.ufoCD = -100, time.Duration(12+e.rng.IntN(10))*time.Second
}

// marchInterval shrinks as the fleet thins out.
func (e *Engine) marchInterval() time.Duration {
	base := 600 * time.Millisecond * time.Duration(e.count) / (rows * cols)
	base -= time.Duration(e.wave-1) * 40 * time.Millisecond
	return max(35*time.Millisecond, base)
}

func (e *Engine) alienPos(r, c int) (int, int) {
	return e.ox + c*spacingX, e.oy + r*spacingY
}

func (e *Engine) playerDown() bool { return e.clock-e.downAt < respawnPause }

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	if e.playerDown() {
		return
	}
	switch k {
	case "left", "a", "h":
		e.player = max(0, e.player-playerStep)
	case "right", "d", "l":
		e.player = min(W-playerW, e.player+playerStep)
	case " ", "up", "w", "enter":
		if e.bullet == nil {
			e.bullet = &shot{x: e.player + playerW/2, y: playerY - 1, up: true}
		}
	}
}

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	if e.state != solo.Playing {
		return
	}
	e.clock += dt
	secs := dt.Seconds()

	// The fleet marches.
	e.march += dt
	for e.march >= e.marchInterval() {
		e.march -= e.marchInterval()
		e.stepFleet()
	}

	// Bombs away.
	e.bombCD -= dt
	if e.bombCD <= 0 && e.count > 0 && len(e.bombs) < 2+e.wave {
		e.dropBomb()
		e.bombCD = max(300*time.Millisecond,
			time.Duration(float64(1300*time.Millisecond)*(0.6+e.rng.Float64()))-time.Duration(e.wave)*60*time.Millisecond)
	}

	// The mystery ship.
	if e.ufo < -50 {
		e.ufoCD -= dt
		if e.ufoCD <= 0 {
			if e.rng.IntN(2) == 0 {
				e.ufo, e.ufoDir = -5, 1
			} else {
				e.ufo, e.ufoDir = W, -1
			}
		}
	} else {
		e.ufo += e.ufoDir * ufoSpeed * secs
		if e.ufo < -6 || e.ufo > W+1 {
			e.ufo, e.ufoCD = -100, time.Duration(15+e.rng.IntN(15))*time.Second
		}
	}

	if b := e.bullet; b != nil {
		for d := bulletSpeed * secs; d > 0 && e.bullet != nil; d -= 1 {
			b.y -= min(1, d)
			e.bulletHits()
		}
	}
	kept := e.bombs[:0]
	for _, b := range e.bombs {
		hit := false
		for d := bombSpeed * secs; d > 0 && !hit; d -= 1 {
			b.y += min(1, d)
			hit = e.bombHits(b)
		}
		if !hit && b.y < H {
			kept = append(kept, b)
		}
	}
	e.bombs = kept

	if e.count == 0 {
		e.wave++
		e.score += 100 * e.wave
		e.startWave()
	}
}

func (e *Engine) stepFleet() {
	e.frame ^= 1
	minX, maxX, maxY := W, 0, 0
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if !e.alive[r][c] {
				continue
			}
			x, y := e.alienPos(r, c)
			minX, maxX, maxY = min(minX, x), max(maxX, x+alienW-1), max(maxY, y+alienH-1)
		}
	}
	if (e.dir > 0 && maxX+1 >= W) || (e.dir < 0 && minX-1 < 0) {
		e.oy += 2
		e.dir = -e.dir
		maxY += 2
	} else {
		e.ox += e.dir
	}
	// Invaders chew through shields they touch, and win if they land.
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if !e.alive[r][c] {
				continue
			}
			x, y := e.alienPos(r, c)
			for dy := 0; dy < alienH; dy++ {
				for dx := 0; dx < alienW; dx++ {
					delete(e.shields, [2]int{x + dx, y + dy})
				}
			}
		}
	}
	if maxY >= playerY {
		e.lives = 0
		e.state = solo.Lost
	}
}

// dropBomb fires from the lowest invader of a random column, preferring the
// column above the player.
func (e *Engine) dropBomb() {
	var candidates []int
	for c := 0; c < cols; c++ {
		for r := rows - 1; r >= 0; r-- {
			if e.alive[r][c] {
				candidates = append(candidates, c)
				break
			}
		}
	}
	if len(candidates) == 0 {
		return
	}
	col := candidates[e.rng.IntN(len(candidates))]
	if e.rng.IntN(8) == 0 {
		// Aim: the column nearest the player.
		best := 1 << 30
		for _, c := range candidates {
			x, _ := e.alienPos(0, c)
			if d := abs(x + 1 - (e.player + playerW/2)); d < best {
				best, col = d, c
			}
		}
	}
	for r := rows - 1; r >= 0; r-- {
		if e.alive[r][col] {
			x, y := e.alienPos(r, col)
			e.bombs = append(e.bombs, &shot{x: x + 1, y: float64(y + alienH)})
			return
		}
	}
}

func (e *Engine) bulletHits() {
	b := e.bullet
	x, y := b.x, int(b.y)
	if y < 0 {
		e.bullet = nil
		return
	}
	if e.shields[[2]int{x, y}] {
		delete(e.shields, [2]int{x, y})
		e.bullet = nil
		return
	}
	if e.ufo > -50 && y <= 2 && x >= int(e.ufo) && x < int(e.ufo)+5 {
		pts := []int{50, 100, 150, 300}[e.rng.IntN(4)]
		e.score += pts
		e.booms = append(e.booms, boom{x: int(e.ufo) + 2, y: 1, at: e.clock, big: true})
		e.ufo, e.ufoCD = -100, time.Duration(15+e.rng.IntN(15))*time.Second
		e.bullet = nil
		return
	}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if !e.alive[r][c] {
				continue
			}
			ax, ay := e.alienPos(r, c)
			if x >= ax && x < ax+alienW && y >= ay && y < ay+alienH {
				e.alive[r][c] = false
				e.count--
				e.score += rowPoints[r]
				e.booms = append(e.booms, boom{x: ax + 1, y: ay, at: e.clock})
				e.bullet = nil
				return
			}
		}
	}
	for i, bm := range e.bombs {
		if bm.x == x && int(bm.y) == y {
			e.bombs = append(e.bombs[:i], e.bombs[i+1:]...)
			e.bullet = nil
			return
		}
	}
}

// bombHits resolves an invader bomb; it reports whether the bomb is used up.
func (e *Engine) bombHits(b *shot) bool {
	x, y := b.x, int(b.y)
	if e.shields[[2]int{x, y}] {
		delete(e.shields, [2]int{x, y})
		delete(e.shields, [2]int{x, y + 1})
		return true
	}
	if !e.playerDown() && y >= playerY && y < playerY+2 && x >= e.player && x < e.player+playerW {
		e.lives--
		e.downAt = e.clock
		e.booms = append(e.booms, boom{x: e.player + 2, y: playerY, at: e.clock, big: true})
		e.bombs = nil
		if e.lives <= 0 {
			e.state = solo.Lost
		}
		return true
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
		{Label: "LIVES", Value: strings.Repeat("▲", max(e.lives, 0))},
		{Label: "WAVE", Value: strconv.Itoa(e.wave)},
	}
}

// View implements solo.Engine.
func (e *Engine) View() string {
	c := canvas.New(W, H)
	for p := range e.shields {
		c.Set(p[0], p[1], theme.Lime)
	}
	for r := 0; r < rows; r++ {
		for col := 0; col < cols; col++ {
			if !e.alive[r][col] {
				continue
			}
			x, y := e.alienPos(r, col)
			f := alienFrames[alienKind(r)][e.frame]
			for dy, line := range f {
				for dx, ch := range line {
					if ch == '#' {
						c.Set(x+dx, y+dy, rowColors[r])
					}
				}
			}
		}
	}
	if e.ufo > -50 {
		x := int(e.ufo)
		c.Fill(x+1, 0, 3, 1, theme.Coral)
		c.Fill(x, 1, 5, 1, theme.Coral)
	}
	if !e.playerDown() || (e.clock/(100*time.Millisecond))%2 == 0 {
		c.Set(e.player+2, playerY, "#FFFFFF")
		c.Fill(e.player, playerY+1, playerW, 1, theme.Lime)
	}
	if b := e.bullet; b != nil {
		c.Set(b.x, int(b.y), "#FFFFFF")
	}
	for _, b := range e.bombs {
		col := theme.Amber
		if (int(b.y)+b.x)%2 == 0 {
			col = theme.Coral
		}
		c.Set(b.x, int(b.y), col)
	}
	for _, b := range e.booms {
		age := e.clock - b.at
		if age > 300*time.Millisecond {
			continue
		}
		r := 1
		if b.big {
			r = 2
		}
		for d := -r; d <= r; d++ {
			c.Set(b.x+d, b.y+d, "#FFE066")
			c.Set(b.x+d, b.y-d, "#FFE066")
		}
	}
	return e.th.Panel.BorderForeground(lipgloss.Color(theme.Violet)).Render(c.Render(e.th.R.ColorProfile()))
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
