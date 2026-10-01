// Package blockfall is a modern falling-blocks game: SRS rotation with wall
// kicks, a 7-bag randomizer, hold, ghost piece, next queue and lock delay.
package blockfall

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

// Board size. Two hidden rows above the visible field give pieces room to
// spawn.
const (
	Cols    = 10
	Rows    = 20
	hidden  = 2
	allRows = Rows + hidden
)

const (
	lockDelay     = 500 * time.Millisecond
	maxLockResets = 15
	clearFlash    = 180 * time.Millisecond
	maxLevel      = 20
)

var lineScores = [5]int{0, 100, 300, 500, 800}

// Game returns the Blockfall game.
func Game() games.Game {
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "blockfall",
			Name:        "Blockfall",
			Icon:        "▦",
			Tagline:     "Stack, spin, clear. Faster and faster.",
			Description: "Guide falling blocks into complete lines. Hold a piece for later, spin it into tight spots with wall kicks, and chase the next level.",
			Kind:        games.Realtime,
			Players:     "1",
			Controls:    []string{"←→", "move", "↑ x", "rotate", "z", "rotate back", "↓", "soft drop", "space", "hard drop", "c", "hold"},
			Accent:      []string{theme.Violet, theme.Cyan, theme.Lime},
			Art: []string{
				"      ████",
				"        ████          ██",
				"  ██        ██  ████████",
				"  ████████████████  ████",
				"  ████  ██████████████████",
			},
		},
		New: func(s games.Session, _ string, rng *rand.Rand) solo.Engine {
			return New(s.Theme, rng)
		},
		Tick: 25 * time.Millisecond,
	})
}

type piece struct {
	k    kind
	rot  int
	x, y int // top-left of the bounding box, in board coordinates
}

func (p piece) cells() [4]cellPos {
	var out [4]cellPos
	for i, c := range shapes[p.k][p.rot] {
		out[i] = cellPos{p.x + c.x, p.y + c.y}
	}
	return out
}

// Engine is one game of Blockfall.
type Engine struct {
	th  *theme.Theme
	rng *rand.Rand

	board [allRows][Cols]int8 // 0 = empty, otherwise kind+1

	cur     piece
	active  bool
	bag     []kind
	next    []kind
	hold    kind
	hasHold bool
	canHold bool

	gravity    time.Duration
	lockTimer  time.Duration
	lockResets int

	clearing []int // rows flashing before removal
	flash    time.Duration

	score, lines, level int
	state               solo.State
}

// New starts a game.
func New(th *theme.Theme, rng *rand.Rand) *Engine {
	e := &Engine{th: th, rng: rng, level: 1, canHold: true}
	for len(e.next) < 3 {
		e.next = append(e.next, e.draw())
	}
	e.spawn(e.pop())
	return e
}

// draw takes the next piece from a shuffled bag of all seven.
func (e *Engine) draw() kind {
	if len(e.bag) == 0 {
		e.bag = []kind{pieceI, pieceJ, pieceL, pieceO, pieceS, pieceT, pieceZ}
		e.rng.Shuffle(len(e.bag), func(i, j int) { e.bag[i], e.bag[j] = e.bag[j], e.bag[i] })
	}
	k := e.bag[0]
	e.bag = e.bag[1:]
	return k
}

func (e *Engine) pop() kind {
	k := e.next[0]
	e.next = append(e.next[1:], e.draw())
	return k
}

func (e *Engine) spawn(k kind) {
	x := 3
	if k == pieceO {
		x = 4
	}
	e.cur = piece{k: k, x: x, y: 0}
	e.active = true
	e.gravity, e.lockTimer, e.lockResets = 0, 0, 0
	if !e.fits(e.cur) {
		e.state = solo.Lost
		return
	}
	// Drop into the visible area right away if there's room.
	if p := e.cur; true {
		p.y++
		if e.fits(p) {
			e.cur = p
		}
	}
}

func (e *Engine) fits(p piece) bool {
	for _, c := range p.cells() {
		if c.x < 0 || c.x >= Cols || c.y < 0 || c.y >= allRows || e.board[c.y][c.x] != 0 {
			return false
		}
	}
	return true
}

func (e *Engine) grounded() bool {
	p := e.cur
	p.y++
	return !e.fits(p)
}

// moved is called after any successful move or rotation: touching a piece
// on the ground buys it more time before it locks, a limited number of times.
func (e *Engine) moved() {
	if e.grounded() && e.lockResets < maxLockResets {
		e.lockTimer = 0
		e.lockResets++
	}
}

func (e *Engine) shift(dx int) bool {
	p := e.cur
	p.x += dx
	if !e.fits(p) {
		return false
	}
	e.cur = p
	e.moved()
	return true
}

func (e *Engine) rotate(dir int) bool {
	if e.cur.k == pieceO {
		return false
	}
	from := e.cur.rot
	to := (from + dir + 4) % 4
	kicks := kicksJLSTZ
	if e.cur.k == pieceI {
		kicks = kicksI
	}
	for _, k := range kicks[[2]int{from, to}] {
		p := e.cur
		p.rot = to
		p.x += k.x
		p.y += k.y
		if e.fits(p) {
			e.cur = p
			e.moved()
			return true
		}
	}
	return false
}

func (e *Engine) fall() bool {
	p := e.cur
	p.y++
	if !e.fits(p) {
		return false
	}
	e.cur = p
	return true
}

func (e *Engine) hardDrop() {
	n := 0
	for e.fall() {
		n++
	}
	e.score += 2 * n
	e.lock()
}

func (e *Engine) holdPiece() {
	if !e.canHold {
		return
	}
	k := e.cur.k
	if e.hasHold {
		e.spawn(e.hold)
	} else {
		e.spawn(e.pop())
	}
	e.hold, e.hasHold, e.canHold = k, true, false
}

func (e *Engine) lock() {
	above := true
	for _, c := range e.cur.cells() {
		e.board[c.y][c.x] = int8(e.cur.k) + 1
		if c.y >= hidden {
			above = false
		}
	}
	e.active = false
	e.canHold = true
	if above { // locked entirely out of sight
		e.state = solo.Lost
		return
	}
	for y := 0; y < allRows; y++ {
		full := true
		for x := 0; x < Cols; x++ {
			if e.board[y][x] == 0 {
				full = false
				break
			}
		}
		if full {
			e.clearing = append(e.clearing, y)
		}
	}
	if len(e.clearing) == 0 {
		e.spawn(e.pop())
		return
	}
	n := len(e.clearing)
	e.score += lineScores[n] * e.level
	e.lines += n
	e.level = min(maxLevel, 1+e.lines/10)
	e.flash = 0
}

// removeCleared drops the flashed rows and spawns the next piece.
func (e *Engine) removeCleared() {
	for _, row := range e.clearing {
		for y := row; y > 0; y-- {
			e.board[y] = e.board[y-1]
		}
		e.board[0] = [Cols]int8{}
	}
	e.clearing = nil
	e.spawn(e.pop())
}

// fallInterval is the time per row at the current level (the modern guideline curve).
func (e *Engine) fallInterval() time.Duration {
	l := float64(e.level - 1)
	secs := math.Pow(0.8-l*0.007, l)
	return time.Duration(secs * float64(time.Second))
}

// Update implements solo.Engine.
func (e *Engine) Update(dt time.Duration) {
	if e.state != solo.Playing {
		return
	}
	if len(e.clearing) > 0 {
		e.flash += dt
		if e.flash >= clearFlash {
			e.removeCleared()
		}
		return
	}
	if !e.active {
		return
	}
	if e.grounded() {
		e.lockTimer += dt
		if e.lockTimer >= lockDelay {
			e.lock()
		}
		return
	}
	e.gravity += dt
	for step := e.fallInterval(); e.gravity >= step; e.gravity -= step {
		if !e.fall() {
			e.gravity = 0
			break
		}
	}
}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	if !e.active || e.state != solo.Playing {
		return
	}
	switch k {
	case "left", "a", "h":
		e.shift(-1)
	case "right", "d", "l":
		e.shift(1)
	case "up", "x", "w", "k":
		e.rotate(1)
	case "z":
		e.rotate(-1)
	case "down", "s", "j":
		if e.fall() {
			e.score++
			e.gravity = 0
		}
	case " ":
		e.hardDrop()
	case "c", "shift+tab", "v":
		e.holdPiece()
	}
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.score }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	return []solo.Stat{
		{Label: "LEVEL", Value: strconv.Itoa(e.level)},
		{Label: "LINES", Value: strconv.Itoa(e.lines)},
	}
}

// ---------------------------------------------------------------------------
// View

const sideW = 12 // width of the side panels, including borders

func (e *Engine) ghost() piece {
	g := e.cur
	for {
		n := g
		n.y++
		if !e.fits(n) {
			return g
		}
		g = n
	}
}

// View implements solo.Engine.
func (e *Engine) View() string {
	board := e.viewBoard()
	left := lipgloss.JoinVertical(lipgloss.Left,
		e.box("HOLD", e.mini(e.hold, e.hasHold, !e.canHold), 4),
		"",
		e.statBlock("LEVEL", strconv.Itoa(e.level)),
		e.statBlock("LINES", strconv.Itoa(e.lines)),
	)
	var next []string
	for i, k := range e.next {
		if i > 0 {
			next = append(next, "")
		}
		next = append(next, e.mini(k, true, false))
	}
	right := e.box("NEXT", strings.Join(next, "\n"), 8)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", board, " ", right)
}

func (e *Engine) viewBoard() string {
	t := e.th
	border := t.Fg(theme.Violet)
	var cells [allRows][Cols]string

	flashing := map[int]bool{}
	for _, r := range e.clearing {
		flashing[r] = true
	}
	for y := hidden; y < allRows; y++ {
		for x := 0; x < Cols; x++ {
			switch v := e.board[y][x]; {
			case flashing[y]:
				cells[y][x] = t.Fg("#FFFFFF").Bold(true).Render("██")
			case v != 0:
				cells[y][x] = t.Fg(colors[v-1]).Render("██")
			default:
				cells[y][x] = t.Faded.Render(" ·")
			}
		}
	}
	if e.active && e.state == solo.Playing {
		g := e.ghost()
		for _, c := range g.cells() {
			if c.y >= hidden {
				cells[c.y][c.x] = t.Fg(colors[g.k]).Faint(true).Render("░░")
			}
		}
	}
	if e.active {
		for _, c := range e.cur.cells() {
			if c.y >= hidden {
				cells[c.y][c.x] = t.Fg(colors[e.cur.k]).Bold(true).Render("██")
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(border.Render("╭" + strings.Repeat("─", Cols*2) + "╮"))
	for y := hidden; y < allRows; y++ {
		sb.WriteString("\n" + border.Render("│"))
		for x := 0; x < Cols; x++ {
			sb.WriteString(cells[y][x])
		}
		sb.WriteString(border.Render("│"))
	}
	sb.WriteString("\n" + border.Render("╰"+strings.Repeat("─", Cols*2)+"╯"))
	return sb.String()
}

// mini draws a piece in two rows, eight columns wide.
func (e *Engine) mini(k kind, show, dim bool) string {
	if !show {
		return strings.Repeat(" ", 8) + "\n" + strings.Repeat(" ", 8)
	}
	// Bounding box of the spawn shape, which is at most 4×2.
	minX, minY, maxX := 4, 4, 0
	for _, c := range spawnShapes[k] {
		minX, minY, maxX = min(minX, c.x), min(minY, c.y), max(maxX, c.x)
	}
	var grid [2][4]bool
	for _, c := range spawnShapes[k] {
		grid[c.y-minY][c.x-minX] = true
	}
	w := maxX - minX + 1
	st := e.th.Fg(colors[k])
	if dim {
		st = e.th.Faded
	}
	pad := strings.Repeat(" ", (8-w*2)/2)
	var rows []string
	for _, r := range grid {
		var sb strings.Builder
		sb.WriteString(pad)
		for _, on := range r[:w] {
			if on {
				sb.WriteString(st.Render("██"))
			} else {
				sb.WriteString("  ")
			}
		}
		rows = append(rows, fitWidth(sb.String(), 8))
	}
	return strings.Join(rows, "\n")
}

func (e *Engine) box(title, content string, innerH int) string {
	t := e.th
	border := t.Fg(theme.Violet)
	inner := sideW - 2
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	label := t.Dim.Bold(true).Render(title)
	sb.WriteString(border.Render("╭─ ") + label + border.Render(" "+strings.Repeat("─", max(0, inner-3-lipgloss.Width(title)))+"╮"))
	for i := 0; i < innerH; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		sb.WriteString("\n" + border.Render("│") + " " + fitWidth(l, inner-1) + border.Render("│"))
	}
	sb.WriteString("\n" + border.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return sb.String()
}

func (e *Engine) statBlock(label, value string) string {
	t := e.th
	return " " + fitWidth(t.Faded.Render(label), sideW-1) + "\n " +
		fitWidth(t.Bold.Render(value), sideW-1)
}

func fitWidth(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
