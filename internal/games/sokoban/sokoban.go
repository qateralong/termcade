// Package sokoban is the warehouse puzzle: push every box onto a goal. Boxes
// can only be pushed, never pulled, and only one at a time.
package sokoban

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"termcade/internal/games"
	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

// Game returns Sokoban.
func Game() games.Game {
	var variants []solo.Variant
	for i, l := range levels {
		lv, err := parse(l.rows)
		boxes := 0
		if err == nil {
			boxes = len(lv.boxes)
		}
		variants = append(variants, solo.Variant{
			Key:  strconv.Itoa(i + 1),
			Name: "Level " + strconv.Itoa(i+1),
			Desc: l.name + " · " + strconv.Itoa(boxes) + " boxes",
		})
	}
	return solo.New(solo.Config{
		Info: games.Info{
			ID:          "sokoban",
			Name:        "Sokoban",
			Icon:        "▣",
			Tagline:     "Push every box onto a goal. You can't pull.",
			Description: "A warehouse puzzle across ten levels. Boxes only move when you push them, one at a time, so think before you shove. Fewest moves wins each level's leaderboard.",
			Kind:        games.TurnBased,
			Players:     "1",
			Controls:    []string{"←↑→↓", "walk / push", "u", "undo", "r", "restart level"},
			Accent:      []string{theme.Amber, theme.Lime, theme.Mint},
			Art: []string{
				"██████████████",
				"██  ◇   ▓▓  ██",
				"██ ▓▓ ◖◗ ◇  ██",
				"██████████████",
			},
		},
		Variants: variants,
		New: func(s games.Session, variant string, _ *rand.Rand) solo.Engine {
			n, _ := strconv.Atoi(variant)
			return New(s.Theme, max(0, n-1))
		},
		LowerIsBetter: true,
		OnlyWins:      true,
		ScoreLabel:    "MOVES",
		FormatScore:   strconv.Itoa,
		Tick:          100 * time.Millisecond,
	})
}

type pos struct{ x, y int }

func (p pos) add(d pos) pos { return pos{p.x + d.x, p.y + d.y} }

// level is a parsed puzzle.
type level struct {
	w, h   int
	walls  map[pos]bool
	goals  map[pos]bool
	boxes  map[pos]bool
	player pos
}

type parseError string

func (e parseError) Error() string { return string(e) }

func parse(rows []string) (*level, error) {
	l := &level{walls: map[pos]bool{}, goals: map[pos]bool{}, boxes: map[pos]bool{}, player: pos{-1, -1}}
	l.h = len(rows)
	for y, row := range rows {
		l.w = max(l.w, len(row))
		for x, c := range row {
			p := pos{x, y}
			switch c {
			case '#':
				l.walls[p] = true
			case '.':
				l.goals[p] = true
			case '$':
				l.boxes[p] = true
			case '*':
				l.boxes[p], l.goals[p] = true, true
			case '@':
				l.player = p
			case '+':
				l.player, l.goals[p] = p, true
			case ' ':
			default:
				return nil, parseError("bad character " + string(c))
			}
		}
	}
	if l.player.x < 0 {
		return nil, parseError("no player")
	}
	if len(l.boxes) != len(l.goals) || len(l.boxes) == 0 {
		return nil, parseError("boxes and goals don't match")
	}
	return l, nil
}

type snapshot struct {
	player pos
	boxes  map[pos]bool
	moves  int
	pushes int
}

// Engine is one level in progress.
type Engine struct {
	th     *theme.Theme
	n      int
	lv     *level
	player pos
	boxes  map[pos]bool
	moves  int
	pushes int
	undo   []snapshot
	state  solo.State
}

// New starts level n (0-based).
func New(th *theme.Theme, n int) *Engine {
	e := &Engine{th: th, n: min(n, len(levels)-1)}
	e.restart()
	return e
}

func (e *Engine) restart() {
	lv, err := parse(levels[e.n].rows)
	if err != nil {
		panic("sokoban: level " + strconv.Itoa(e.n+1) + ": " + err.Error())
	}
	e.lv = lv
	e.player = lv.player
	e.boxes = map[pos]bool{}
	for b := range lv.boxes {
		e.boxes[b] = true
	}
	e.moves, e.pushes, e.undo = 0, 0, nil
}

func copyBoxes(m map[pos]bool) map[pos]bool {
	out := make(map[pos]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// step walks or pushes in direction d.
func (e *Engine) step(d pos) {
	next := e.player.add(d)
	if e.lv.walls[next] {
		return
	}
	snap := snapshot{player: e.player, boxes: copyBoxes(e.boxes), moves: e.moves, pushes: e.pushes}
	if e.boxes[next] {
		beyond := next.add(d)
		if e.lv.walls[beyond] || e.boxes[beyond] {
			return
		}
		delete(e.boxes, next)
		e.boxes[beyond] = true
		e.pushes++
	}
	e.undo = append(e.undo, snap)
	e.player = next
	e.moves++
	if e.solved() {
		e.state = solo.Won
	}
}

func (e *Engine) solved() bool {
	for b := range e.boxes {
		if !e.lv.goals[b] {
			return false
		}
	}
	return true
}

// Update implements solo.Engine.
func (e *Engine) Update(time.Duration) {}

// Key implements solo.Engine.
func (e *Engine) Key(k string) {
	switch k {
	case "up", "w", "k":
		e.step(pos{0, -1})
	case "down", "s", "j":
		e.step(pos{0, 1})
	case "left", "a", "h":
		e.step(pos{-1, 0})
	case "right", "d", "l":
		e.step(pos{1, 0})
	case "u", "backspace", "z":
		if n := len(e.undo); n > 0 {
			s := e.undo[n-1]
			e.undo = e.undo[:n-1]
			e.player, e.boxes, e.moves, e.pushes = s.player, s.boxes, s.moves, s.pushes
		}
	case "r":
		e.restart()
	}
}

// Score implements solo.Engine.
func (e *Engine) Score() int { return e.moves }

// State implements solo.Engine.
func (e *Engine) State() solo.State { return e.state }

// Stats implements solo.Engine.
func (e *Engine) Stats() []solo.Stat {
	done := 0
	for b := range e.boxes {
		if e.lv.goals[b] {
			done++
		}
	}
	return []solo.Stat{
		{Label: "PUSHES", Value: strconv.Itoa(e.pushes)},
		{Label: "BOXES", Value: strconv.Itoa(done) + "/" + strconv.Itoa(len(e.boxes))},
	}
}

// View implements solo.Engine.
func (e *Engine) View() string {
	t := e.th
	wall := t.Fg("#5A4F80")
	var sb strings.Builder
	sb.WriteString(t.Dim.Bold(true).Render("LEVEL "+strconv.Itoa(e.n+1)) + t.Faded.Render("  "+levels[e.n].name))
	for y := 0; y < e.lv.h; y++ {
		sb.WriteString("\n")
		for x := 0; x < e.lv.w; x++ {
			p := pos{x, y}
			switch {
			case e.lv.walls[p]:
				sb.WriteString(wall.Render("██"))
			case p == e.player:
				sb.WriteString(t.Fg(theme.Pink).Bold(true).Render("◖◗"))
			case e.boxes[p] && e.lv.goals[p]:
				sb.WriteString(t.Fg(theme.Lime).Bold(true).Render("▓▓"))
			case e.boxes[p]:
				sb.WriteString(t.Fg("#C9874A").Render("▓▓"))
			case e.lv.goals[p]:
				sb.WriteString(t.Fg(theme.Amber).Render("◇ "))
			default:
				sb.WriteString("  ")
			}
		}
	}
	return sb.String()
}
