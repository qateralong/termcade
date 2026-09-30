package tanks

// Maps are 30×18 tiles; each tile is 2×2 pixels. They are built from the
// top-left quadrant (15×9), mirrored both ways, so every spawn corner is
// equally good. Legend:
//
//	.  empty   #  brick (destructible)   @  steel
//	~  water (blocks tanks, not shells)  %  bushes (hide tanks)
var quadrants = []struct {
	name string
	rows [9]string
}{
	{"Crossroads", [9]string{
		"...............",
		"..##..##..##...",
		"..##..##..##...",
		"..##..##..##..@",
		".........%%....",
		"@@...##.......~",
		"......##....~~~",
		"..##......##...",
		"..##..@@..##...",
	}},
	{"Fortress", [9]string{
		"...............",
		"..@@@..####....",
		"..@....#.......",
		"..@..%%#..##...",
		"......%%..##...",
		".####.........#",
		".#......~~~...#",
		".#..##..~~~....",
		"....##.........",
	}},
	{"Jungle", [9]string{
		"...............",
		"..%%%%...##....",
		"..%%%%...##..%%",
		"......##.....%%",
		"..##..##..@@...",
		"..##......@@...",
		"......%%%......",
		"..~~..%%%..##..",
		"..~~.......##..",
	}},
}

const (
	TilesW = 30
	TilesH = 18
)

// buildMap mirrors a quadrant into a full tile map.
func buildMap(q [9]string) [TilesH][TilesW]byte {
	var m [TilesH][TilesW]byte
	for y := 0; y < TilesH; y++ {
		qy := y
		if y >= 9 {
			qy = TilesH - 1 - y
		}
		for x := 0; x < TilesW; x++ {
			qx := x
			if x >= 15 {
				qx = TilesW - 1 - x
			}
			m[y][x] = q[qy][qx]
		}
	}
	return m
}

// Spawn corners, as top-left tiles of a 2×2 tile area.
var spawns = [4][2]int{{0, 0}, {TilesW - 2, TilesH - 2}, {TilesW - 2, 0}, {0, TilesH - 2}}
