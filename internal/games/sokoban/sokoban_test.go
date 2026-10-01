package sokoban

import (
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"termcade/internal/games/solo"
	"termcade/internal/ui/theme"
)

var dirs = []pos{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

// solve finds the fewest moves to solve a level by breadth-first search,
// or -1 if it can't be solved within the state budget.
func solve(l *level) int {
	key := func(p pos, boxes map[pos]bool) string {
		var bs []string
		for b := range boxes {
			bs = append(bs, strconv.Itoa(b.x)+","+strconv.Itoa(b.y))
		}
		sort.Strings(bs)
		return strconv.Itoa(p.x) + "," + strconv.Itoa(p.y) + "|" + strings.Join(bs, ";")
	}
	type state struct {
		p     pos
		boxes map[pos]bool
		depth int
	}
	start := state{l.player, l.boxes, 0}
	seen := map[string]bool{key(start.p, start.boxes): true}
	queue := []state{start}
	for len(queue) > 0 && len(seen) < 2_000_000 {
		s := queue[0]
		queue = queue[1:]
		done := true
		for b := range s.boxes {
			if !l.goals[b] {
				done = false
				break
			}
		}
		if done {
			return s.depth
		}
		for _, d := range dirs {
			n := s.p.add(d)
			if l.walls[n] {
				continue
			}
			boxes := s.boxes
			if s.boxes[n] {
				beyond := n.add(d)
				if l.walls[beyond] || s.boxes[beyond] {
					continue
				}
				boxes = copyBoxes(s.boxes)
				delete(boxes, n)
				boxes[beyond] = true
			}
			k := key(n, boxes)
			if !seen[k] {
				seen[k] = true
				queue = append(queue, state{n, boxes, s.depth + 1})
			}
		}
	}
	return -1
}

func TestEveryLevelIsSolvable(t *testing.T) {
	for i, l := range levels {
		lv, err := parse(l.rows)
		if err != nil {
			t.Fatalf("level %d (%s): %v", i+1, l.name, err)
		}
		n := solve(lv)
		if n < 0 {
			t.Errorf("level %d (%s) can't be solved", i+1, l.name)
			continue
		}
		t.Logf("level %d (%s): %d boxes, best solution %d moves", i+1, l.name, len(lv.boxes), n)
	}
}

func TestPushRules(t *testing.T) {
	e := New(theme.New(lipgloss.NewRenderer(io.Discard)), 0)
	// "# @$ .#": walking right pushes the box onto the goal in two moves.
	e.Key("right")
	if e.pushes != 1 || e.moves != 1 {
		t.Fatalf("moves=%d pushes=%d", e.moves, e.pushes)
	}
	e.Key("right")
	if e.state != solo.Won || e.Score() != 2 {
		t.Fatalf("state=%v score=%d", e.state, e.Score())
	}
	e.Key("u")
	e.Key("u")
	if e.player != (pos{2, 2}) || e.moves != 0 {
		t.Fatalf("undo: player %v moves %d", e.player, e.moves)
	}
	// Can't push into a wall.
	e.Key("up")
	e.Key("up")
	if e.player != (pos{2, 1}) {
		t.Fatalf("walked through a wall: %v", e.player)
	}
}

func TestViewHeights(t *testing.T) {
	th := theme.New(lipgloss.NewRenderer(io.Discard))
	for i := range levels {
		e := New(th, i)
		if h := lipgloss.Height(e.View()); h != e.lv.h+1 {
			t.Errorf("level %d view height %d", i+1, h)
		}
	}
}
