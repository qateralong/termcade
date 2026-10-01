package blockfall

import "termcade/internal/ui/theme"

// kind is a tetromino type.
type kind int

const (
	pieceI kind = iota
	pieceJ
	pieceL
	pieceO
	pieceS
	pieceT
	pieceZ
	numKinds
)

type cellPos struct{ x, y int }

// spawnShapes are the cells of each piece in rotation state 0, inside its
// bounding box (4×4 for I, 2×2 for O, 3×3 otherwise), with y growing down.
var spawnShapes = [numKinds][4]cellPos{
	pieceI: {{0, 1}, {1, 1}, {2, 1}, {3, 1}},
	pieceJ: {{0, 0}, {0, 1}, {1, 1}, {2, 1}},
	pieceL: {{2, 0}, {0, 1}, {1, 1}, {2, 1}},
	pieceO: {{0, 0}, {1, 0}, {0, 1}, {1, 1}},
	pieceS: {{1, 0}, {2, 0}, {0, 1}, {1, 1}},
	pieceT: {{1, 0}, {0, 1}, {1, 1}, {2, 1}},
	pieceZ: {{0, 0}, {1, 0}, {1, 1}, {2, 1}},
}

var boxSize = [numKinds]int{pieceI: 4, pieceO: 2, pieceJ: 3, pieceL: 3, pieceS: 3, pieceT: 3, pieceZ: 3}

var colors = [numKinds]string{
	pieceI: theme.Cyan,
	pieceJ: theme.Sky,
	pieceL: "#FF9F43",
	pieceO: theme.Amber,
	pieceS: theme.Lime,
	pieceT: theme.Violet,
	pieceZ: theme.Coral,
}

// shapes[k][r] are the cells of piece k in rotation r (0, R, 2, L).
var shapes [numKinds][4][4]cellPos

func init() {
	for k := kind(0); k < numKinds; k++ {
		n := boxSize[k]
		shapes[k][0] = spawnShapes[k]
		for r := 1; r < 4; r++ {
			for i, c := range shapes[k][r-1] {
				// Clockwise rotation within the bounding box. This matches
				// the Super Rotation System's basic rotation.
				shapes[k][r][i] = cellPos{n - 1 - c.y, c.x}
			}
		}
	}
}

// Wall kick offsets from the Super Rotation System, converted to y-down.
// Indexed by [from][to]; only adjacent rotations are used.
var kicksJLSTZ = map[[2]int][5]cellPos{
	{0, 1}: {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
	{1, 0}: {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
	{1, 2}: {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
	{2, 1}: {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
	{2, 3}: {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
	{3, 2}: {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
	{3, 0}: {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
	{0, 3}: {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
}

var kicksI = map[[2]int][5]cellPos{
	{0, 1}: {{0, 0}, {-2, 0}, {1, 0}, {-2, 1}, {1, -2}},
	{1, 0}: {{0, 0}, {2, 0}, {-1, 0}, {2, -1}, {-1, 2}},
	{1, 2}: {{0, 0}, {-1, 0}, {2, 0}, {-1, -2}, {2, 1}},
	{2, 1}: {{0, 0}, {1, 0}, {-2, 0}, {1, 2}, {-2, -1}},
	{2, 3}: {{0, 0}, {2, 0}, {-1, 0}, {2, -1}, {-1, 2}},
	{3, 2}: {{0, 0}, {-2, 0}, {1, 0}, {-2, 1}, {1, -2}},
	{3, 0}: {{0, 0}, {1, 0}, {-2, 0}, {1, 2}, {-2, -1}},
	{0, 3}: {{0, 0}, {-1, 0}, {2, 0}, {-1, -2}, {2, 1}},
}
