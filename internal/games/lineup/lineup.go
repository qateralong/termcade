// Package lineup assembles the games shown in the lobby, in display order.
package lineup

import (
	"termcade/internal/games"
	"termcade/internal/games/minesweeper"
	"termcade/internal/games/pacman"
	"termcade/internal/games/snake"
	"termcade/internal/games/tetris"
)

// All returns every game: solo games first, then multiplayer.
func All() []games.Game {
	solo := []games.Game{
		pacman.Game(),
		tetris.Game(),
		snake.Game(),
		minesweeper.Game(),
	}
	return append(solo, games.Upcoming()...)
}
