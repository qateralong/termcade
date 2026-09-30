package pacman

import (
	"strings"
)

// The maze is 28×22 cells. Legend:
//
//	#  wall        .  dot          o  power pellet
//	-  ghost door  G  ghost house  P  Pac-Man's start
//	   (space) empty corridor; rows open at both edges are tunnels
var layout = []string{
	"############################",
	"#............##............#",
	"#.####.#####.##.#####.####.#",
	"#o####.#####.##.#####.####o#",
	"#..........................#",
	"#.####.##.########.##.####.#",
	"#......##....##....##......#",
	"######.##### ## #####.######",
	"######.##          ##.######",
	"######.## ###--### ##.######",
	"      .   #GGGGGG#   .      ",
	"######.## ######## ##.######",
	"######.##          ##.######",
	"######.## ######## ##.######",
	"#............##............#",
	"#.####.#####.##.#####.####.#",
	"#o..##.......P........##..o#",
	"###.##.##.########.##.##.###",
	"#......##....##....##......#",
	"#.##########.##.##########.#",
	"#..........................#",
	"############################",
}

// Maze dimensions.
var (
	MazeW = len(layout[0])
	MazeH = len(layout)
)

type tile byte

const (
	tileOpen tile = iota
	tileWall
	tileDoor
	tileHouse
)

type point struct{ x, y int }

// Fixed places in the maze.
var (
	pacStart  point
	ghostExit = point{13, 8}  // just above the door
	houseSpot = point{13, 10} // where eaten ghosts go to revive
	readyRow  = 12            // where "READY!" is written
)

type maze struct {
	tiles [][]tile
	dots  [][]byte // 0 none, 1 dot, 2 power pellet
	total int      // dots and pellets at the start of a level
}

func newMaze() *maze {
	m := &maze{}
	for y, row := range layout {
		tr := make([]tile, MazeW)
		dr := make([]byte, MazeW)
		for x, c := range row {
			switch c {
			case '#':
				tr[x] = tileWall
			case '-':
				tr[x] = tileDoor
			case 'G':
				tr[x] = tileHouse
			case '.':
				dr[x] = 1
				m.total++
			case 'o':
				dr[x] = 2
				m.total++
			case 'P':
				pacStart = point{x, y}
			}
		}
		m.tiles = append(m.tiles, tr)
		m.dots = append(m.dots, dr)
	}
	return m
}

func (m *maze) at(p point) tile {
	if p.y < 0 || p.y >= MazeH {
		return tileWall
	}
	p.x = wrapX(p.x)
	return m.tiles[p.y][p.x]
}

func wrapX(x int) int { return (x%MazeW + MazeW) % MazeW }

// open reports whether p is walkable space outside the ghost house.
func (m *maze) open(p point) bool { return m.at(p) == tileOpen }

// wallGlyphs renders the maze walls once. Walls are drawn as outlines: only
// the edges that face open space are drawn, so thick blocks come out hollow.
func wallGlyphs(m *maze) [][]string {
	isWall := func(x, y int) bool {
		if y < 0 || y >= MazeH || x < 0 || x >= MazeW {
			return false
		}
		return m.tiles[y][x] == tileWall
	}
	isSpace := func(x, y int) bool {
		if y < 0 || y >= MazeH || x < 0 || x >= MazeW {
			return false
		}
		return m.tiles[y][x] != tileWall
	}
	out := make([][]string, MazeH)
	for y := 0; y < MazeH; y++ {
		out[y] = make([]string, MazeW)
		for x := 0; x < MazeW; x++ {
			if !isWall(x, y) {
				continue
			}
			// Connect to a neighboring wall when the edge between them
			// borders open space on either side.
			right := isWall(x+1, y) && (isSpace(x, y-1) || isSpace(x+1, y-1) || isSpace(x, y+1) || isSpace(x+1, y+1))
			left := isWall(x-1, y) && (isSpace(x, y-1) || isSpace(x-1, y-1) || isSpace(x, y+1) || isSpace(x-1, y+1))
			down := isWall(x, y+1) && (isSpace(x-1, y) || isSpace(x-1, y+1) || isSpace(x+1, y) || isSpace(x+1, y+1))
			up := isWall(x, y-1) && (isSpace(x-1, y) || isSpace(x-1, y-1) || isSpace(x+1, y) || isSpace(x+1, y-1))

			var g string
			switch {
			case up && down && left && right:
				g = "┼"
			case up && down && right:
				g = "├"
			case up && down && left:
				g = "┤"
			case left && right && down:
				g = "┬"
			case left && right && up:
				g = "┴"
			case right && down:
				g = "╭"
			case left && down:
				g = "╮"
			case right && up:
				g = "╰"
			case left && up:
				g = "╯"
			case left || right:
				g = "─"
			case up || down:
				g = "│"
			default:
				g = " "
			}
			tail := " "
			if right {
				tail = "─"
			}
			out[y][x] = g + tail
		}
	}
	return out
}

// mazeString is the raw layout, handy in tests.
func mazeString() string { return strings.Join(layout, "\n") }
